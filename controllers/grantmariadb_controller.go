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

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/controllers/template"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GrantMariaDBReconciler reconciles a GrantMariaDB object
type GrantMariaDBReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=grantmariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=grantmariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=grantmariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GrantMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var grant databasev1alpha1.GrantMariaDB
	if err := r.Get(ctx, req.NamespacedName, &grant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	wr := newWrappedGrantReconciler(r.Client, *r.RefResolver, &grant)
	wf := newWrappedGrantFinalizer(r.Client, &grant)
	tf := template.NewTemplateFinalizer(r.RefResolver, wf)
	tr := template.NewTemplateReconciler(r.RefResolver, r.ConditionReady, wr, tf)

	result, err := tr.Reconcile(ctx, &grant)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.GrantMariaDB{}).
		Complete(r)
}

type wrappedGrantReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	grant       *databasev1alpha1.GrantMariaDB
}

func newWrappedGrantReconciler(client client.Client, refResolver refresolver.RefResolver,
	grant *databasev1alpha1.GrantMariaDB) template.WrappedReconciler {
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
		return fmt.Errorf("error patching GrantMariaDB status: %v", err)
	}
	return nil
}

func userKey(grant *databasev1alpha1.GrantMariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      grant.Spec.Username,
		Namespace: grant.Namespace,
	}
}
