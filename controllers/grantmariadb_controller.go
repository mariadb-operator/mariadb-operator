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
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	grantFinalizerName = "grant.database.mmontes.io/finalizer"
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

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariadb, r.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB client: %v", err)
	}
	defer mdbClient.Close()

	if grant.IsBeingDeleted() {
		if err := r.finalize(ctx, &grant, mdbClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing GrantMariaDB: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.addFinalizer(ctx, &grant); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to GrantMariaDB: %v", err)
	}

	var grantErr *multierror.Error
	err = r.grant(ctx, &grant, mdbClient)
	grantErr = multierror.Append(grantErr, err)

	err = r.patchStatus(ctx, &grant, conditions.NewConditionReadyPatcher(err))
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
	return mdbClient.Grant(ctx, opts)
}

func (r *GrantMariaDBReconciler) addFinalizer(ctx context.Context, grant *databasev1alpha1.GrantMariaDB) error {
	if controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(grant.DeepCopy())
	controllerutil.AddFinalizer(grant, grantFinalizerName)
	return r.Client.Patch(ctx, grant, patch)
}

func (r *GrantMariaDBReconciler) finalize(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	mdbClient *mariadbclient.Client) error {
	if !controllerutil.ContainsFinalizer(grant, grantFinalizerName) {
		return nil
	}

	if err := r.revoke(ctx, grant, mdbClient); err != nil {
		return fmt.Errorf("error revoking grant: %v", err)
	}

	patch := ctrlClient.MergeFrom(grant.DeepCopy())
	controllerutil.RemoveFinalizer(grant, grantFinalizerName)
	return r.Client.Patch(ctx, grant, patch)
}

func (r *GrantMariaDBReconciler) revoke(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	mdbClient *mariadbclient.Client) error {
	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Second, func(ctx context.Context) (bool, error) {
		var user databasev1alpha1.UserMariaDB
		if err := r.Get(ctx, userKey(grant), &user); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
	// User does not exist
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, wait.ErrWaitTimeout) {
		return fmt.Errorf("error checking if user exists: %v", err)
	}

	opts := mariadbclient.GrantOpts{
		Privileges:  grant.Spec.Privileges,
		Database:    grant.Spec.Database,
		Table:       grant.Spec.Table,
		Username:    grant.Spec.Username,
		GrantOption: grant.Spec.GrantOption,
	}
	if err := mdbClient.Revoke(ctx, opts); err != nil {
		return fmt.Errorf("error revoking grants in MariaDB: %v", err)
	}
	return nil
}

func (r *GrantMariaDBReconciler) patchStatus(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	patcher conditions.ConditionPatcher) error {
	patch := client.MergeFrom(grant.DeepCopy())
	patcher(&grant.Status)
	return r.Client.Status().Patch(ctx, grant, patch)
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
