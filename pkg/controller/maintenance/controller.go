package maintenance

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type MaintenanceReconciler struct {
	client.Client
	recorder    events.EventRecorder
	refResolver *refresolver.RefResolver
}

func NewMaintenanceReconciler(client client.Client, recorder events.EventRecorder) *MaintenanceReconciler {
	r := &MaintenanceReconciler{
		Client:   client,
		recorder: recorder,
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	return r
}

func (r *MaintenanceReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	// We are in an early reconciliation stage, skipping. Pods are still coming up, new events will be triggered.
	if mariadb.Status.CurrentPrimary == nil || mariadb.Status.CurrentPrimaryPodIndex == nil {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("maintenance")

	if !mariadb.IsMaintenanceModeEnabled() {
		return r.reconcileReadOnly(ctx, mariadb, logger)
	}

	if result, err := r.reconcileDrainConnections(ctx, mariadb, logger); !result.IsZero() || err != nil {
		return result, err
	}
	if result, err := r.reconcileReadOnly(ctx, mariadb, logger); !result.IsZero() || err != nil {
		return result, err
	}
	return ctrl.Result{}, nil
}
