package endpoints

import (
	"context"
	"errors"
	"fmt"
	"sort"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	mdbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoAddressesAvailable = errors.New("no addresses available")

type EndpointsReconciler struct {
	client.Client
	builder *builder.Builder
}

func NewEndpointsReconciler(client client.Client, builder *builder.Builder) *EndpointsReconciler {
	return &EndpointsReconciler{
		Client:  client,
		builder: builder,
	}
}

func (r *EndpointsReconciler) Reconcile(ctx context.Context, key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB) error {
	desiredEndpoints, err := r.endpoints(ctx, key, mariadb)
	if err != nil {
		if errors.Is(err, ErrNoAddressesAvailable) {
			return err
		}
		return fmt.Errorf("error building desired Endpoints: %v", err)
	}

	var existingEndpoints corev1.Endpoints
	if err := r.Get(ctx, key, &existingEndpoints); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Endpoints: %v", err)
		}
		if err := r.Create(ctx, desiredEndpoints); err != nil {
			return fmt.Errorf("error creating Endpoints: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingEndpoints.DeepCopy())
	existingEndpoints.Subsets = desiredEndpoints.Subsets
	return r.Patch(ctx, &existingEndpoints, patch)
}

func (r *EndpointsReconciler) endpoints(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB) (*corev1.Endpoints, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, fmt.Errorf("'status.currentPrimaryPodIndex' must be set")
	}
	podList := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDB(mariadb).
				Build(),
		),
		Namespace: mariadb.GetNamespace(),
	}
	if err := r.List(ctx, &podList, listOpts); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].Status.PodIP < podList.Items[j].Status.PodIP
	})

	addresses := []corev1.EndpointAddress{}
	notReadyAddresses := []corev1.EndpointAddress{}
	for _, pod := range podList.Items {
		addr := endpointAddress(&pod)
		if addr == nil {
			continue
		}
		podIndex, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting Pod '%s' index: %v", pod.Name, err)
		}
		if *podIndex == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}

		if mdbpod.PodReady(&pod) {
			addresses = append(addresses, *addr)
		} else {
			notReadyAddresses = append(notReadyAddresses, *addr)
		}
	}
	if len(addresses) == 0 && len(notReadyAddresses) == 0 {
		return nil, ErrNoAddressesAvailable
	}

	subsets := []corev1.EndpointSubset{
		{
			Addresses:         addresses,
			NotReadyAddresses: notReadyAddresses,
			Ports: []corev1.EndpointPort{
				{
					Name:     builder.MariaDbPortName,
					Port:     mariadb.Spec.Port,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
	endpoints, err := r.builder.BuildEndpoints(key, mariadb, subsets)
	if err != nil {
		return nil, fmt.Errorf("error building Endpoints: %v", err)
	}
	return endpoints, nil
}

func endpointAddress(pod *corev1.Pod) *corev1.EndpointAddress {
	if pod.Status.PodIP == "" || pod.Spec.NodeName == "" {
		return nil
	}
	return &corev1.EndpointAddress{
		IP:       pod.Status.PodIP,
		NodeName: &pod.Spec.NodeName,
		TargetRef: &corev1.ObjectReference{
			Kind:      "Pod",
			Name:      pod.Name,
			Namespace: pod.Namespace,
			UID:       pod.UID,
		},
	}
}
