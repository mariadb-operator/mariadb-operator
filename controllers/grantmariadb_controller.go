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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
)

// GrantMariaDBReconciler reconciles a GrantMariaDB object
type GrantMariaDBReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RefResolver *refresolver.RefResolver
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

	mariadb, err := r.RefResolver.GetMariaDB(ctx, grant.Spec.MariaDBRef, grant.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", err)
	}

	client, err := mariadbclient.NewRootClientWithCrd(ctx, mariadb, r.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB client: %v", err)
	}
	defer client.Close()

	err = r.grant(ctx, &grant, client)
	if patchErr := r.patchGrantStatus(ctx, &grant, err); patchErr != nil {
		return ctrl.Result{}, fmt.Errorf("error patching GrantMariaDB status: %v", err)
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating GrantMariaDB: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *GrantMariaDBReconciler) grant(ctx context.Context, grant *databasev1alpha1.GrantMariaDB, client *mariadbclient.Client) error {
	params := mariadbclient.GrantParams{
		Privileges: grant.Spec.Privileges,
		Database:   grant.Spec.Database,
		Table:      grant.Spec.Table,
		Username:   grant.Spec.User.Username,
	}
	var identifiedBy string
	if grant.Spec.User.PasswordSecretKeyRef != nil {
		identifiedBy, _ = r.RefResolver.ReadSecretKeyRef(ctx, *grant.Spec.User.PasswordSecretKeyRef, grant.Namespace)
	}
	opts := mariadbclient.GrantOpts{
		IdentifiedBy:       identifiedBy,
		GrantOption:        grant.Spec.GrantOption,
		MaxUserConnections: grant.Spec.MaxUserConnections,
	}
	return client.Grant(ctx, params, opts)
}

func (r *GrantMariaDBReconciler) patchGrantStatus(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	err error) error {
	patch := client.MergeFrom(grant.DeepCopy())
	conditions.AddConditionReady(&grant.Status, err)
	return r.Client.Status().Patch(ctx, grant, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.GrantMariaDB{}).
		Complete(r)
}
