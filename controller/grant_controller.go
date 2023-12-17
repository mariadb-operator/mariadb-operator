package controller

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/sql"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/pkg/sql"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	usernameField = ".spec.username"
)

// GrantReconciler reconciles a Grant object
type GrantReconciler struct {
	client.Client
	RefResolver     *refresolver.RefResolver
	ConditionReady  *condition.Ready
	RequeueInterval time.Duration
}

func NewGrantReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *condition.Ready,
	requeueInterval time.Duration) *GrantReconciler {
	return &GrantReconciler{
		Client:          client,
		RefResolver:     refResolver,
		ConditionReady:  conditionReady,
		RequeueInterval: requeueInterval,
	}
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=grants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=grants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=grants/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var grant mariadbv1alpha1.Grant
	if err := r.Get(ctx, req.NamespacedName, &grant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	wr := newWrappedGrantReconciler(r.Client, *r.RefResolver, &grant)
	wf := newWrappedGrantFinalizer(r.Client, &grant)
	tf := sql.NewSqlFinalizer(r.Client, wf)
	tr := sql.NewSqlReconciler(r.Client, r.ConditionReady, wr, tf, r.RequeueInterval)

	result, err := tr.Reconcile(ctx, &grant)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.createIndex(mgr); err != nil {
		return fmt.Errorf("error creating index: %v", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.Grant{}).
		Watches(
			&mariadbv1alpha1.User{},
			handler.EnqueueRequestsFromMapFunc(r.mapUserToRequests),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(ce event.CreateEvent) bool {
					return true
				},
			}),
		).
		Complete(r)
}

func (r *GrantReconciler) createIndex(mgr ctrl.Manager) error {
	indexFn := func(rawObj client.Object) []string {
		grant := rawObj.(*mariadbv1alpha1.Grant)
		if grant.Spec.Username == "" {
			return nil
		}
		return []string{grant.Spec.Username}
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &mariadbv1alpha1.Grant{}, usernameField, indexFn); err != nil {
		return fmt.Errorf("error indexing '%s' field in Grant: %v", usernameField, err)
	}
	return nil
}

func (r *GrantReconciler) mapUserToRequests(ctx context.Context, user client.Object) []reconcile.Request {
	grantsToReconcile := &mariadbv1alpha1.GrantList{}
	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(usernameField, user.GetName()),
		Namespace:     user.GetNamespace(),
	}

	if err := r.List(context.Background(), grantsToReconcile, listOpts); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(grantsToReconcile.Items))
	for i, item := range grantsToReconcile.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

type wrappedGrantReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	grant       *mariadbv1alpha1.Grant
}

func newWrappedGrantReconciler(client client.Client, refResolver refresolver.RefResolver,
	grant *mariadbv1alpha1.Grant) sql.WrappedReconciler {
	return &wrappedGrantReconciler{
		Client:      client,
		refResolver: &refResolver,
		grant:       grant,
	}
}

func (wr *wrappedGrantReconciler) Reconcile(ctx context.Context, mdbClient *sqlClient.Client) error {
	var opts []sqlClient.GrantOption
	if wr.grant.Spec.GrantOption {
		opts = append(opts, sqlClient.WithGrantOption())
	}
	if err := mdbClient.Grant(
		ctx,
		wr.grant.Spec.Privileges,
		wr.grant.Spec.Database,
		wr.grant.Spec.Table,
		wr.grant.AccountName(),
		opts...,
	); err != nil {
		return fmt.Errorf("error granting privileges in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedGrantReconciler) PatchStatus(ctx context.Context, patcher condition.Patcher) error {
	patch := client.MergeFrom(wr.grant.DeepCopy())
	patcher(&wr.grant.Status)

	if err := wr.Client.Status().Patch(ctx, wr.grant, patch); err != nil {
		return fmt.Errorf("error patching Grant status: %v", err)
	}
	return nil
}

func userKey(grant *mariadbv1alpha1.Grant) types.NamespacedName {
	return types.NamespacedName{
		Name:      grant.Spec.Username,
		Namespace: grant.Namespace,
	}
}
