package endpoints

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	kadapter "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/adapter"
	mdbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var errNoEndpointsAvailable = errors.New("no endpoints available")

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

func (r *EndpointsReconciler) Reconcile(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB, serviceName string) (ctrl.Result, error) {
	logger := log.FromContext(ctx).V(1).WithName("endpoints")

	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		logger.Info("'status.currentPrimaryPodIndex' must be set")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	desiredEndpointSlice, err := r.endpointSlice(ctx, key, mariadb, serviceName, logger)
	if err != nil {
		if errors.Is(err, errNoEndpointsAvailable) {
			logger.Info("No endpoints available. Requeing...")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error building desired EndpointSlice: %v", err)
	}

	var existingEndpointSlice discoveryv1.EndpointSlice
	if err := r.Get(ctx, key, &existingEndpointSlice); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting EndpointSlice: %v", err)
		}
		if err := r.Create(ctx, desiredEndpointSlice); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating EndpointSlice: %v", err)
		}
		return ctrl.Result{}, nil
	}

	patch := client.MergeFrom(existingEndpointSlice.DeepCopy())
	existingEndpointSlice.Endpoints = desiredEndpointSlice.Endpoints
	existingEndpointSlice.Ports = desiredEndpointSlice.Ports
	return ctrl.Result{}, r.Patch(ctx, &existingEndpointSlice, patch)
}

func (r *EndpointsReconciler) endpointSlice(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB, serviceName string, logger logr.Logger) (*discoveryv1.EndpointSlice, error) {
	podList := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				Build(),
		),
		Namespace: mariadb.GetNamespace(),
	}
	if err := r.List(ctx, &podList, listOpts); err != nil {
		return nil, fmt.Errorf("error listing Pods: %v", err)
	}
	if len(podList.Items) == 0 {
		return nil, errNoEndpointsAvailable
	}
	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].Status.PodIP < podList.Items[j].Status.PodIP
	})

	addressType, err := getAddressType(&podList.Items[0])
	if err != nil {
		logger.Info("error getting address type", "err", err)
		return nil, errNoEndpointsAvailable
	}

	endpoints := []discoveryv1.Endpoint{}
	for _, pod := range podList.Items {
		endpoint, err := buildEndpoint(&pod)
		if err != nil {
			logger.Info("error building Endpoint", "err", err)
			continue
		}

		podIndex, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting Pod '%s' index: %v", pod.Name, err)
		}
		if *podIndex == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		endpoints = append(endpoints, *endpoint)
	}
	if len(endpoints) == 0 {
		return nil, errNoEndpointsAvailable
	}

	ports := []discoveryv1.EndpointPort{
		{
			Name:     ptr.To(builder.MariadbPortName),
			Port:     ptr.To(mariadb.Spec.Port),
			Protocol: ptr.To(corev1.ProtocolTCP),
		},
	}
	if mariadb.Spec.ServicePorts != nil {
		for _, servicePort := range kadapter.ToKubernetesSlice(mariadb.Spec.ServicePorts) {
			ports = append(ports, discoveryv1.EndpointPort{
				Name:     ptr.To(servicePort.Name),
				Port:     ptr.To(servicePort.Port),
				Protocol: ptr.To(corev1.ProtocolTCP),
			})
		}
	}

	endpointSlice, err := r.builder.BuildEndpointSlice(key, mariadb, *addressType, endpoints, ports, serviceName)
	if err != nil {
		return nil, fmt.Errorf("error building EndpointSlice: %v", err)
	}
	return endpointSlice, nil
}

func buildEndpoint(pod *corev1.Pod) (*discoveryv1.Endpoint, error) {
	if pod.Status.PodIP == "" || pod.Spec.NodeName == "" {
		return nil, errors.New("Pod IP and Nodename must be set")
	}
	return &discoveryv1.Endpoint{
		Addresses: []string{pod.Status.PodIP},
		NodeName:  &pod.Spec.NodeName,
		TargetRef: &corev1.ObjectReference{
			Kind:      "Pod",
			Name:      pod.Name,
			Namespace: pod.Namespace,
			UID:       pod.UID,
		},
		Conditions: discoveryv1.EndpointConditions{
			Ready: ptr.To(mdbpod.PodReady(pod)),
		},
	}, nil
}

func getAddressType(pod *corev1.Pod) (*discoveryv1.AddressType, error) {
	if pod.Status.PodIP == "" {
		return nil, errors.New("Pod IP and Nodename must be set")
	}
	parsedIp := net.ParseIP(pod.Status.PodIP)
	if parsedIp == nil {
		return nil, fmt.Errorf("error parsing Pod IP address: %v", pod.Status.PodIP)
	}
	if parsedIp.To4() != nil {
		return ptr.To(discoveryv1.AddressTypeIPv4), nil
	}
	return ptr.To(discoveryv1.AddressTypeIPv6), nil
}
