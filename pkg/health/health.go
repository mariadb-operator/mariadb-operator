package health

import (
	"context"
	"errors"
	"fmt"
	"sort"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func IsStatefulSetHealthy(ctx context.Context, client ctrlclient.Client, key types.NamespacedName,
	opts ...HealthOpt) (bool, error) {
	var sts appsv1.StatefulSet
	if err := client.Get(ctx, key, &sts); err != nil {
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

	var endpoints corev1.Endpoints
	if err := client.Get(ctx, key, &endpoints); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			if port.Port == *healthOpts.Port {
				switch *healthOpts.EndpointPolicy {
				case EndpointPolicyAll:
					return len(subset.Addresses) == int(healthOpts.DesiredReplicas), nil
				case EndpointPolicyAtLeastOne:
					return len(subset.Addresses) > 0, nil
				default:
					return false, fmt.Errorf("unsupported EndpointPolicy '%v'", *healthOpts.EndpointPolicy)
				}
			}
		}
	}
	return false, nil
}

func HealthyMariaDBReplica(ctx context.Context, client client.Client, mariadb *mariadbv1alpha1.MariaDB) (*int, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	podList := corev1.PodList{}
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				Build(),
		),
		Namespace: mariadb.GetNamespace(),
	}
	if err := client.List(ctx, &podList, listOpts); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	sortPodList(podList)

	for _, p := range podList.Items {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if *index == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		if pod.PodReady(&p) {
			return index, nil
		}
	}
	return nil, ErrNoHealthyInstancesAvailable
}

func HealthyMaxScalePod(ctx context.Context, client client.Client, maxscale *mariadbv1alpha1.MaxScale) (*int, error) {
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
	sortPodList(podList)

	for _, p := range podList.Items {
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting index for Pod '%s': %v", p.Name, err)
		}
		if pod.PodReady(&p) {
			return index, nil
		}
	}
	return nil, ErrNoHealthyInstancesAvailable
}

func IsServiceHealthy(ctx context.Context, client client.Client, serviceKey types.NamespacedName) (bool, error) {
	var endpoints v1.Endpoints
	err := client.Get(ctx, serviceKey, &endpoints)
	if err != nil {
		return false, err
	}
	if len(endpoints.Subsets) == 0 {
		return false, fmt.Errorf("'%s/%s' subsets not ready", serviceKey.Name, serviceKey.Namespace)
	}
	if len(endpoints.Subsets[0].Addresses) == 0 {
		return false, fmt.Errorf("'%s/%s' addresses not ready", serviceKey.Name, serviceKey.Namespace)
	}
	return true, nil
}

func sortPodList(list corev1.PodList) {
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].Name < list.Items[j].Name
	})
}
