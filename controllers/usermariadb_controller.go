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

	"github.com/hashicorp/go-multierror"
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	userFinalizerName = "user.database.mmontes.io/finalizer"
)

// UserMariaDBReconciler reconciles a UserMariaDB object
type UserMariaDBReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=usermariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=usermariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=usermariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var user databasev1alpha1.UserMariaDB
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, ctrlClient.IgnoreNotFound(err)
	}

	var mariaDbErr *multierror.Error
	mariaDb, err := r.RefResolver.GetMariaDB(ctx, user.Spec.MariaDBRef, user.Namespace)
	if err != nil {
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &user, r.ConditionReady.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	var connErr *multierror.Error
	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariaDb, r.RefResolver)
	if err != nil {
		connErr = multierror.Append(connErr, err)

		err = r.patchStatus(ctx, &user, r.ConditionReady.FailedPatcher("Error connecting to MariaDB"))
		connErr = multierror.Append(connErr, err)

		return ctrl.Result{}, fmt.Errorf("error creating MariaDB client: %v", connErr)
	}
	defer mdbClient.Close()

	if user.IsBeingDeleted() {
		if err := r.finalize(ctx, &user, mdbClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing UserMariaDB: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.addFinalizer(ctx, &user); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to UserMariaDB: %v", err)
	}

	var userErr *multierror.Error
	err = r.createUser(ctx, &user, mdbClient)
	userErr = multierror.Append(userErr, err)

	err = r.patchStatus(ctx, &user, r.ConditionReady.PatcherWithError(err))
	userErr = multierror.Append(userErr, err)

	if err := userErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating UserMariaDB: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *UserMariaDBReconciler) createUser(ctx context.Context, user *databasev1alpha1.UserMariaDB, mdbClient *mariadbclient.Client) error {
	password, err := r.RefResolver.ReadSecretKeyRef(ctx, user.Spec.PasswordSecretKeyRef, user.Namespace)
	if err != nil {
		return fmt.Errorf("error reading user password secret: %v", err)
	}
	opts := mariadbclient.CreateUserOpts{
		IdentifiedBy:       password,
		MaxUserConnections: user.Spec.MaxUserConnections,
	}

	if err := mdbClient.CreateUser(ctx, user.Name, opts); err != nil {
		return fmt.Errorf("error creating user in MariaDB: %v", err)
	}
	return nil
}

func (r *UserMariaDBReconciler) addFinalizer(ctx context.Context, user *databasev1alpha1.UserMariaDB) error {
	if controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(user.DeepCopy())
	controllerutil.AddFinalizer(user, userFinalizerName)

	if err := r.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error adding finalizer to UserMariaDB: %v", err)
	}
	return nil
}

func (r *UserMariaDBReconciler) finalize(ctx context.Context, user *databasev1alpha1.UserMariaDB,
	mdbClient *mariadbclient.Client) error {
	if !controllerutil.ContainsFinalizer(user, userFinalizerName) {
		return nil
	}

	if err := mdbClient.DropUser(ctx, user.Name); err != nil {
		return fmt.Errorf("error dropping user in MariaDB: %v", err)
	}

	patch := ctrlClient.MergeFrom(user.DeepCopy())
	controllerutil.RemoveFinalizer(user, userFinalizerName)

	if err := r.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error removing finalizer to UserMariaDB: %v", err)
	}
	return nil
}

func (r *UserMariaDBReconciler) patchStatus(ctx context.Context, user *databasev1alpha1.UserMariaDB,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(user.DeepCopy())
	patcher(&user.Status)

	if err := r.Client.Status().Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error patching UserMariaDB status: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.UserMariaDB{}).
		Complete(r)
}
