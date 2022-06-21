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

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

// RestoreMariaDBReconciler reconciles a RestoreMariaDB object
type RestoreMariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RestoreMariaDB object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *RestoreMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var restore databasev1alpha1.RestoreMariaDB
	if err := r.Get(ctx, req.NamespacedName, &restore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var job batchv1.Job
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting Job: %v", err)
		}

		mariadb, err := r.getMariaDB(ctx, &restore)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", err)
		}
		backup, err := r.getBackup(ctx, &restore)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting BackupMariaDB: %v", err)
		}

		if err := r.createJob(ctx, &restore, mariadb, backup); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *RestoreMariaDBReconciler) createJob(ctx context.Context, restore *databasev1alpha1.RestoreMariaDB,
	mariadb *databasev1alpha1.MariaDB, backup *databasev1alpha1.BackupMariaDB) error {
	return nil
}

func (r *RestoreMariaDBReconciler) getMariaDB(ctx context.Context,
	restore *databasev1alpha1.RestoreMariaDB) (*databasev1alpha1.MariaDB, error) {
	var mariadb databasev1alpha1.MariaDB
	nn := types.NamespacedName{
		Name:      restore.Spec.MariaDBRef.Name,
		Namespace: restore.Namespace,
	}
	if err := r.Get(ctx, nn, &mariadb); err != nil {
		return nil, fmt.Errorf("error getting MariaDB on API server: %v", err)
	}
	return &mariadb, nil
}

func (r *RestoreMariaDBReconciler) getBackup(ctx context.Context,
	restore *databasev1alpha1.RestoreMariaDB) (*databasev1alpha1.BackupMariaDB, error) {
	var backup databasev1alpha1.BackupMariaDB
	nn := types.NamespacedName{
		Name:      restore.Spec.BackupMariaDBRef.Name,
		Namespace: restore.Namespace,
	}
	if err := r.Get(ctx, nn, &backup); err != nil {
		return nil, fmt.Errorf("error getting BackupMariaDB on API server: %v", err)
	}
	return &backup, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.RestoreMariaDB{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
