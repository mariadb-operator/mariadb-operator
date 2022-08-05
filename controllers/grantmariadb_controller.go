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
	"time"

	"github.com/hashicorp/go-multierror"
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
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

	if grant.IsBeingDeleted() {
		if err := r.finalize(ctx, &grant); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing GrantMariaDB: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.addFinalizer(ctx, &grant); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to GrantMariaDB: %v", err)
	}

	var mariaDbErr *multierror.Error
	mariaDb, err := r.RefResolver.GetMariaDB(ctx, grant.Spec.MariaDBRef, grant.Namespace)
	if err != nil {
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &grant, r.ConditionReady.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if !mariaDb.IsReady() {
		if err := r.patchStatus(ctx, &grant, r.ConditionReady.FailedPatcher("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching GrantMariaDB: %v", err)
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	var connErr *multierror.Error
	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, r.RefResolver)
	if err != nil {
		connErr = multierror.Append(connErr, err)

		err = r.patchStatus(ctx, &grant, r.ConditionReady.FailedPatcher("Error connecting to MariaDB"))
		connErr = multierror.Append(connErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating MariaDB client: %v", connErr)
	}
	defer mdbClient.Close()

	var grantErr *multierror.Error
	err = r.grant(ctx, &grant, mdbClient)
	grantErr = multierror.Append(grantErr, err)

	err = r.patchStatus(ctx, &grant, r.ConditionReady.PatcherWithError(err))
	grantErr = multierror.Append(grantErr, err)

	if err := grantErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating GrantMariaDB: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *GrantMariaDBReconciler) grant(ctx context.Context, grant *databasev1alpha1.GrantMariaDB, mdbClient *mariadbclient.Client) error {
	opts := mariadbclient.GrantOpts{
		Privileges:  grant.Spec.Privileges,
		Database:    grant.Spec.Database,
		Table:       grant.Spec.Table,
		Username:    grant.Spec.Username,
		GrantOption: grant.Spec.GrantOption,
	}
	if err := mdbClient.Grant(ctx, opts); err != nil {
		return fmt.Errorf("error granting privileges in MariaDB: %v", err)
	}
	return nil
}

func (r *GrantMariaDBReconciler) patchStatus(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(grant.DeepCopy())
	patcher(&grant.Status)

	if err := r.Client.Status().Patch(ctx, grant, patch); err != nil {
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

// SetupWithManager sets up the controller with the Manager.
func (r *GrantMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.GrantMariaDB{}).
		Complete(r)
}
