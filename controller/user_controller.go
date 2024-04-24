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
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *condition.Ready
	SqlOpts        []sql.SqlOpt
}

func NewUserReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *condition.Ready,
	sqlOpts ...sql.SqlOpt) *UserReconciler {
	return &UserReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		SqlOpts:        sqlOpts,
	}
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var user mariadbv1alpha1.User
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, ctrlClient.IgnoreNotFound(err)
	}

	wr := newWrapperUserReconciler(r.Client, r.RefResolver, &user)
	wf := newWrappedUserFinalizer(r.Client, &user)
	tf := sql.NewSqlFinalizer(r.Client, wf, r.SqlOpts...)
	tr := sql.NewSqlReconciler(r.Client, r.ConditionReady, wr, tf, r.SqlOpts...)

	result, err := tr.Reconcile(ctx, &user)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.User{}).
		Complete(r)
}

type wrappedUserReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	user        *mariadbv1alpha1.User
}

func newWrapperUserReconciler(client client.Client, refResolver *refresolver.RefResolver,
	user *mariadbv1alpha1.User) sql.WrappedReconciler {
	return &wrappedUserReconciler{
		Client:      client,
		refResolver: refResolver,
		user:        user,
	}
}

func (wr *wrappedUserReconciler) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	password, err := wr.refResolver.SecretKeyRef(ctx, wr.user.Spec.PasswordSecretKeyRef, wr.user.Namespace)
	if err != nil {
		return fmt.Errorf("error reading user password secret: %v", err)
	}

	opts := sqlClient.CreateUserOpts{
		IdentifiedBy:       password,
		MaxUserConnections: wr.user.Spec.MaxUserConnections,
	}
	if err := mdbClient.CreateUser(ctx, wr.user.AccountName(), opts); err != nil {
		return fmt.Errorf("error creating user in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedUserReconciler) PatchStatus(ctx context.Context, patcher condition.Patcher) error {
	patch := client.MergeFrom(wr.user.DeepCopy())
	patcher(&wr.user.Status)

	if err := wr.Client.Status().Patch(ctx, wr.user, patch); err != nil {
		return fmt.Errorf("error patching User status: %v", err)
	}
	return nil
}
