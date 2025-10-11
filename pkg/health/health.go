package health

import (
	"context"
	"errors"
	"fmt"
	"sort"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	mdbpod "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoHealthyInstancesAvailable = errors.New("no healthy instances available")

type EndpointPolicy string

const (
	EndpointPolicyAll        EndpointPolicy = "All"
	EndpointPolicyAtLeastOne EndpointPolicy = "AtLeastOne"
)

type HealthOpts struct {
	DesiredReplicas int32
	Port            *int32
	EndpointPolicy  *EndpointPolicy
}

type HealthOpt func(*HealthOpts)

func WithDesiredReplicas(r int32) HealthOpt {
	return func(ho *HealthOpts) {
		ho.DesiredReplicas = r
	}
}

func WithPort(p int32) HealthOpt {
	return func(ho *HealthOpts) {
		ho.Port = ptr.To(p)
	}
}

func WithEndpointPolicy(e EndpointPolicy) HealthOpt {
	return func(ho *HealthOpts) {
		ho.EndpointPolicy = ptr.To(e)
	}
}

func IsStatefulSetHealthy(ctx context.Context, client ctrlclient.Client, serviceKey types.NamespacedName,
	opts ...HealthOpt) (bool, error) {
	var sts appsv1.StatefulSet
	if err := client.Get(ctx, serviceKey, &sts); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}

	healthOpts := HealthOpts{
		DesiredReplicas: ptr.Deref(sts.Spec.Replicas, 1),
		EndpointPolicy:  ptr.To(EndpointPolicyAll),
	}
	for _, setOpt := range opts {
		setOpt(&healthOpts)
	}

	if sts.Status.ReadyReplicas != healthOpts.DesiredReplicas {
		return false, nil
	}
	if healthOpts.Port == nil || healthOpts.EndpointPolicy == nil {
		return true, nil
	}

	endpointSliceList := discoveryv1.EndpointSliceList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			map[string]string{
				metadata.KubernetesServiceLabel: serviceKey.Name,
			},
		),
		Namespace: serviceKey.Namespace,
	}
	if err := client.List(ctx, &endpointSliceList, listOpts); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}
	for _, endpointSlice := range endpointSliceList.Items {
		matchesPort := false
		for _, port := range endpointSlice.Ports {
			if port.Port != nil && *port.Port == *healthOpts.Port {
				matchesPort = true
				break
			}
		}
		if !matchesPort {
			continue
		}

		readyEndpoints := 0
		for _, endpoint := range endpointSlice.Endpoints {
			if ptr.Deref(endpoint.Conditions.Ready, false) {
				readyEndpoints++
			}
		}
		switch *healthOpts.EndpointPolicy {
		case EndpointPolicyAll:
			return readyEndpoints == int(healthOpts.DesiredReplicas), nil
		case EndpointPolicyAtLeastOne:
			return readyEndpoints > 0, nil
		default:
			return false, fmt.Errorf("unsupported EndpointPolicy '%v'", *healthOpts.EndpointPolicy)
		}

	}
	return false, nil
}

func SecondaryPodHealthyIndex(ctx context.Context, client ctrlclient.Client, mariadb *mariadbv1alpha1.MariaDB) (*int, error) {
	return secondaryPodHealthyIndex(ctx, client, mariadb, func(p *corev1.Pod) bool {
		return mdbpod.PodReady(p)
	})
}

func ReplicaPodHealthyIndex(ctx context.Context, client ctrlclient.Client, mariadb *mariadbv1alpha1.MariaDB) (*int, error) {
	return secondaryPodHealthyIndex(ctx, client, mariadb, func(p *corev1.Pod) bool {
		return mdbpod.PodReady(p) && mariadb.IsConfiguredReplica(p.Name)
	})
}

func secondaryPodHealthyIndex(ctx context.Context, client ctrlclient.Client, mariadb *mariadbv1alpha1.MariaDB,
	isHealthy func(*corev1.Pod) bool) (*int, error) {
	pods, err := mdbpod.ListMariaDBSecondaryPods(ctx, client, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	sortPods(pods)

	for _, p := range pods {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if isHealthy(&p) {
			return index, nil
		}
	}
	return nil, ErrNoHealthyInstancesAvailable
}

func HealthyMaxScalePod(ctx context.Context, client ctrlclient.Client, maxscale *mariadbv1alpha1.MaxScale) (*int, error) {
	podList := corev1.PodList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMaxScaleSelectorLabels(maxscale).
				Build(),
		),
		Namespace: maxscale.GetNamespace(),
	}
	if err := client.List(ctx, &podList, listOpts); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	sortPodList(&podList)

	for _, p := range podList.Items {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if mdbpod.PodReady(&p) {
			return index, nil
		}
	}
	return nil, ErrNoHealthyInstancesAvailable
}

func IsServiceHealthy(ctx context.Context, client ctrlclient.Client, serviceKey types.NamespacedName) (bool, error) {
	endpointSliceList := discoveryv1.EndpointSliceList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			map[string]string{
				metadata.KubernetesServiceLabel: serviceKey.Name,
			},
		),
		Namespace: serviceKey.Namespace,
	}
	if err := client.List(ctx, &endpointSliceList, listOpts); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}
	for _, endpointSlice := range endpointSliceList.Items {
		readyEndpoints := 0
		for _, endpoint := range endpointSlice.Endpoints {
			if ptr.Deref(endpoint.Conditions.Ready, false) {
				readyEndpoints++
			}
		}
		if readyEndpoints > 0 {
			return true, nil
		}
	}
	return false, nil
}

func sortPods(pods []corev1.Pod) {
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})
}

func sortPodList(list *corev1.PodList) {
	sortPods(list.Items)
}
