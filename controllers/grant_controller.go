/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/controller/sql"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	usernameField = ".spec.username"
)

// GrantReconciler reconciles a Grant object
type GrantReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
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
	tf := sql.NewSqlFinalizer(r.RefResolver, wf)
	tr := sql.NewSqlReconciler(r.RefResolver, r.ConditionReady, wr, tf)

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
			&source.Kind{Type: &mariadbv1alpha1.User{}},
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

func (r *GrantReconciler) mapUserToRequests(user client.Object) []reconcile.Request {
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

func (wr *wrappedGrantReconciler) Reconcile(ctx context.Context, mdbClient *mariadbclient.Client) error {
	opts := mariadbclient.GrantOpts{
		Privileges:  wr.grant.Spec.Privileges,
		Database:    wr.grant.Spec.Database,
		Table:       wr.grant.Spec.Table,
		Username:    wr.grant.Spec.Username,
		GrantOption: wr.grant.Spec.GrantOption,
	}
	if err := mdbClient.Grant(ctx, opts); err != nil {
		return fmt.Errorf("error granting privileges in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedGrantReconciler) PatchStatus(ctx context.Context, patcher conditions.Patcher) error {
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
