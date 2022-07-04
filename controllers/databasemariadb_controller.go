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
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
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
		return ctrl.Result{}, fmt.Errorf("error getting MariaDB client: %v", err)
	}
	defer mdbClient.Close()

	if database.IsBeingDeleted() {
		if err := r.finalizeDatabase(ctx, &database, mdbClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("error finalizing DatabaseMariaDB: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.addDatabaseFinalizer(ctx, &database); err != nil {
		return ctrl.Result{}, fmt.Errorf("error adding finalizer to DatabaseMariaDB: %v", err)
	}

	err = r.createDatabase(ctx, &database, mdbClient)
	if patchErr := r.patchDatabaseStatus(ctx, &database, err); patchErr != nil {
		return ctrl.Result{}, fmt.Errorf("error patching DatabaseMariaDB status: %v", err)
	}
	if err != nil {
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
	return mdbClient.CreateDatabase(ctx, database.Name, opts)
}

func (r *DatabaseMariaDBReconciler) patchDatabaseStatus(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	err error) error {
	patch := client.MergeFrom(database.DeepCopy())
	conditions.AddConditionReady(&database.Status, err)
	return r.Client.Status().Patch(ctx, database, patch)
}

func (r *DatabaseMariaDBReconciler) addDatabaseFinalizer(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB) error {
	if controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}
	patch := ctrlClient.MergeFrom(database.DeepCopy())
	controllerutil.AddFinalizer(database, databaseFinalizerName)
	return r.Client.Patch(ctx, database, patch)
}

func (r *DatabaseMariaDBReconciler) finalizeDatabase(ctx context.Context, database *databasev1alpha1.DatabaseMariaDB,
	mdbClient *mariadbclient.Client) error {
	if !controllerutil.ContainsFinalizer(database, databaseFinalizerName) {
		return nil
	}

	if err := mdbClient.DropDatabase(ctx, database.Name); err != nil {
		return fmt.Errorf("error dropping database in MariaDB: %v", err)
	}

	patch := ctrlClient.MergeFrom(database.DeepCopy())
	controllerutil.RemoveFinalizer(database, databaseFinalizerName)
	return r.Client.Patch(ctx, database, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.DatabaseMariaDB{}).
		Complete(r)
}
