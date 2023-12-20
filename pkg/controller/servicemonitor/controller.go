package servicemonitor

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceMonitorReconciler struct {
	client.Client
}

func NewServiceMonitorReconciler(client client.Client) *ServiceMonitorReconciler {
	return &ServiceMonitorReconciler{
		Client: client,
	}
}

func (r *ServiceMonitorReconciler) Reconcile(ctx context.Context, desiredSvcMonitor *monitoringv1.ServiceMonitor) error {
	key := client.ObjectKeyFromObject(desiredSvcMonitor)
	var existingSvcMonitor monitoringv1.ServiceMonitor
	if err := r.Get(ctx, key, &existingSvcMonitor); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting ServiceMonitor: %v", err)
		}
		if err := r.Create(ctx, desiredSvcMonitor); err != nil {
			return fmt.Errorf("error creating ServiceMonitor: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingSvcMonitor.DeepCopy())
	existingSvcMonitor.Spec.Endpoints = desiredSvcMonitor.Spec.Endpoints
	return r.Patch(ctx, &existingSvcMonitor, patch)
}
