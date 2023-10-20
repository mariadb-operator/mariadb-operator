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
	existingSvc.Spec.Ports = desiredSvc.Spec.Ports
	existingSvc.Spec.Selector = desiredSvc.Spec.Selector
	existingSvc.Spec.Type = desiredSvc.Spec.Type
	for k, v := range desiredSvc.Annotations {
		existingSvc.Annotations[k] = v
	}
	for k, v := range desiredSvc.Labels {
		existingSvc.Labels[k] = v
	}

	return r.Patch(ctx, &existingSvc, patch)
}
