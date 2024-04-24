package controller

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/sql"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/pkg/sql"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready
	SqlOpts        []sql.SqlOpt
}

func NewDatabaseReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *condition.Ready,
	sqlOpts ...sql.SqlOpt) *DatabaseReconciler {
	return &DatabaseReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		SqlOpts:        sqlOpts,
	}
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=databases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=databases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=databases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var database mariadbv1alpha1.Database
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	wr := newWrappedDatabaseReconciler(r.Client, r.RefResolver, &database)
	wf := newWrappedDatabaseFinalizer(r.Client, &database)
	tf := sql.NewSqlFinalizer(r.Client, wf, r.SqlOpts...)
	tr := sql.NewSqlReconciler(r.Client, r.ConditionReady, wr, tf, r.SqlOpts...)

	result, err := tr.Reconcile(ctx, &database)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.Database{}).
		Complete(r)
}

type wrappedDatabaseReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	database    *mariadbv1alpha1.Database
}

func newWrappedDatabaseReconciler(client client.Client, refResolver *refresolver.RefResolver,
	database *mariadbv1alpha1.Database) sql.WrappedReconciler {
	return &wrappedDatabaseReconciler{
		Client:      client,
		refResolver: refResolver,
		database:    database,
	}
}

func (wr *wrappedDatabaseReconciler) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	opts := sqlClient.DatabaseOpts{
		CharacterSet: wr.database.Spec.CharacterSet,
		Collate:      wr.database.Spec.Collate,
	}
	if err := mdbClient.CreateDatabase(ctx, wr.database.DatabaseNameOrDefault(), opts); err != nil {
		return fmt.Errorf("error creating database in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedDatabaseReconciler) PatchStatus(ctx context.Context, patcher condition.Patcher) error {
	patch := client.MergeFrom(wr.database.DeepCopy())
	patcher(&wr.database.Status)

	if err := wr.Client.Status().Patch(ctx, wr.database, patch); err != nil {
		return fmt.Errorf("error patching Database status: %v", err)
	}
	return nil
}
