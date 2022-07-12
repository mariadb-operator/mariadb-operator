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
	"github.com/mmontes11/mariadb-operator/pkg/builders"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// RestoreMariaDBReconciler reconciles a RestoreMariaDB object
type RestoreMariaDBReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RefResolver *refresolver.RefResolver
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

	err := r.createJob(ctx, &restore, req.NamespacedName)
	if patchErr := r.patchStatus(ctx, &restore, conditions.NewConditionCreatedPatcher(err)); patchErr != nil {
		return ctrl.Result{}, fmt.Errorf("error patching RestoreMariaDB status: %v", patchErr)
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *RestoreMariaDBReconciler) createJob(ctx context.Context, restore *databasev1alpha1.RestoreMariaDB,
	key types.NamespacedName) error {
	var existingJob batchv1.Job
	if err := r.Get(ctx, key, &existingJob); err == nil {
		return nil
	}

	mariadb, err := r.RefResolver.GetMariaDB(ctx, restore.Spec.MariaDBRef, restore.Namespace)
	if err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	backup, err := r.RefResolver.GetBackupMariaDB(ctx, restore.Spec.BackupRef, restore.Namespace)
	if err != nil {
		return fmt.Errorf("error getting BackupMariaDB: %v", err)
	}

	job := builders.BuildRestoreJob(restore, mariadb, backup, key)
	if err := controllerutil.SetControllerReference(restore, job, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to Job: %v", err)
	}

	if err := r.Create(ctx, job); err != nil {
		return fmt.Errorf("error creating Job on API server: %v", err)
	}
	return nil
}

func (r *RestoreMariaDBReconciler) patchStatus(ctx context.Context, restore *databasev1alpha1.RestoreMariaDB,
	patcher conditions.ConditionPatcher) error {
	patch := client.MergeFrom(restore.DeepCopy())
	patcher(&restore.Status)
	return r.Client.Status().Patch(ctx, restore, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.RestoreMariaDB{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
