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
	databaseFinalizerName = "database.database.mmontes.io/finalizer"
)

// DatabaseMariaDBReconciler reconciles a DatabaseMariaDB object
type DatabaseMariaDBReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RefResolver *refresolver.RefResolver
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=databasemariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=databasemariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=databasemariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DatabaseMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var database databasev1alpha1.DatabaseMariaDB
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, err := r.RefResolver.GetMariaDB(ctx, database.Spec.MariaDBRef, database.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", err)
	}

	mdbClient, err := mariadbclient.NewRootClientWithCrd(ctx, mariadb, r.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating MariaDB client: %v", err)
	}
	defer mdbClient.Close()

	if database.IsBeingDeleted() {
		if err := r.finalize(ctx, &database, mdbClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing DatabaseMariaDB: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.addFinalizer(ctx, &database); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to DatabaseMariaDB: %v", err)
	}

	var databaseErr *multierror.Error
	err = r.createDatabase(ctx, &database, mdbClient)
	databaseErr = multierror.Append(databaseErr, err)

	err = r.patchStatus(ctx, &database, conditions.NewConditionReadyPatcher(err))
	databaseErr = multierror.Append(databaseErr, err)

	if err := databaseErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating DatabaseMariaDB: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *DatabaseMariaDBReconciler) createDatabase(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	mdbClient *mariadbclient.Client) error {
	opts := mariadbclient.DatabaseOpts{
		CharacterSet: database.Spec.CharacterSet,
		Collate:      database.Spec.Collate,
	}
	if err := mdbClient.CreateDatabase(ctx, database.Name, opts); err != nil {
		return fmt.Errorf("error creating database in MariaDB: %v", err)
	}
	return nil
}

func (r *DatabaseMariaDBReconciler) addFinalizer(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB) error {
	if controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(database.DeepCopy())
	controllerutil.AddFinalizer(database, databaseFinalizerName)

	if err := r.Client.Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error adding finalizer to DatabaseMariaDB: %v", err)
	}
	return nil
}

func (r *DatabaseMariaDBReconciler) finalize(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	mdbClient *mariadbclient.Client) error {
	if !controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}

	if err := mdbClient.DropDatabase(ctx, database.Name); err != nil {
		return fmt.Errorf("error dropping database in MariaDB: %v", err)
	}

	patch := ctrlClient.MergeFrom(database.DeepCopy())
	controllerutil.RemoveFinalizer(database, databaseFinalizerName)

	if err := r.Client.Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error removing finalizer to DatabaseMariaDB: %v", err)
	}
	return nil
}

func (r *DatabaseMariaDBReconciler) patchStatus(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	patcher conditions.ConditionPatcher) error {
	patch := client.MergeFrom(database.DeepCopy())
	patcher(&database.Status)

	if err := r.Client.Status().Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error patching DatabaseMariaDB status: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.DatabaseMariaDB{}).
		Complete(r)
}
