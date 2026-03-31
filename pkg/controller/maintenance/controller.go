package maintenance

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type MaintenanceReconciler struct {
	client.Client
	recorder          events.EventRecorder
	builder           *builder.Builder
	refResolver       *refresolver.RefResolver
	serviceReconciler *service.ServiceReconciler
}

func NewMaintenanceReconciler(client client.Client, recorder events.EventRecorder, builder *builder.Builder) *MaintenanceReconciler {
	r := &MaintenanceReconciler{
		Client:   client,
		recorder: recorder,
		builder:  builder,
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewServiceReconciler(client)
	}
	return r
}

func (r *MaintenanceReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsMaintenanceModeEnabled() {
		return ctrl.Result{}, nil // TODO: branch needed for readonly
	}
	logger := log.FromContext(ctx).WithName("maintenance")

	clientSet := sql.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

	if result, err := r.reconcileDrainConnections(ctx, mariadb, clientSet, logger); !result.IsZero() || err != nil {
		return result, err
	}

	return ctrl.Result{}, nil
}
