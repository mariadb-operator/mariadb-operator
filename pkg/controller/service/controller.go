package service

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceReconciler struct {
	client.Client
}

func NewServiceReconciler(client client.Client) *ServiceReconciler {
	return &ServiceReconciler{
		Client: client,
	}
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, desiredSvc *corev1.Service) error {
	key := client.ObjectKeyFromObject(desiredSvc)
	var existingSvc corev1.Service
	if err := r.Get(ctx, key, &existingSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Service: %v", err)
		}
		if err := r.Create(ctx, desiredSvc); err != nil {
			return fmt.Errorf("error creating Service: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingSvc.DeepCopy())
	updateServicePorts(&existingSvc, desiredSvc)
	existingSvc.Spec.AllocateLoadBalancerNodePorts = desiredSvc.Spec.AllocateLoadBalancerNodePorts
	existingSvc.Spec.Selector = desiredSvc.Spec.Selector
	existingSvc.Spec.Type = desiredSvc.Spec.Type

	if existingSvc.Annotations == nil {
		existingSvc.Annotations = make(map[string]string)
	}
	for k, v := range desiredSvc.Annotations {
		existingSvc.Annotations[k] = v
	}
	if existingSvc.Labels == nil {
		existingSvc.Labels = make(map[string]string)
	}
	for k, v := range desiredSvc.Labels {
		existingSvc.Labels[k] = v
	}

	return r.Patch(ctx, &existingSvc, patch)
}

// updateServicePorts updates the ports of an existing service based on desired service ports.
// If the existing service has no ports, it assigns the desired service's ports to it.
// If the existing service has ports, it compares them with the desired service ports and performs necessary updates.
func updateServicePorts(existingSvc, desiredSvc *corev1.Service) {
	if existingSvc == nil || desiredSvc == nil {
		return
	}

	if len(existingSvc.Spec.Ports) == 0 {
		existingSvc.Spec.Ports = desiredSvc.Spec.Ports
		return
	}

	existingPorts := make(map[int32]bool)
	for _, port := range existingSvc.Spec.Ports {
		existingPorts[port.Port] = true
	}

	for _, desiredPort := range desiredSvc.Spec.Ports {
		if !existingPorts[desiredPort.Port] {
			existingSvc.Spec.Ports = append(existingSvc.Spec.Ports, desiredPort)
		}
	}

	if desiredSvc.Spec.Type != corev1.ServiceTypeNodePort && desiredSvc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		for i := range existingSvc.Spec.Ports {
			existingSvc.Spec.Ports[i].NodePort = 0
		}
	}
}
