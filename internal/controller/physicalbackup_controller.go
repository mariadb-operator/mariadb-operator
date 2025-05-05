package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PhysicalBackupReconciler reconciles a PhysicalBackup object
type PhysicalBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PhysicalBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PhysicalBackupReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.PhysicalBackup{}).
		Owns(&batchv1.Job{})

	if err := mariadbv1alpha1.IndexPhysicalBackup(ctx, mgr, builder, r.Client); err != nil {
		return fmt.Errorf("error indexing PhysicalBackup: %v", err)
	}

	return builder.Complete(r)
}
