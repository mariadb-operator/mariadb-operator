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
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/controller/job"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestoreMariaDBReconciler reconciles a RestoreMariaDB object
type RestoreMariaDBReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ConditionComplete *conditions.Complete
	JobReconciler     *job.JobReconciler
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RestoreMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var restore databasev1alpha1.RestoreMariaDB
	if err := r.Get(ctx, req.NamespacedName, &restore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariaDb, err := r.RefResolver.GetMariaDB(ctx, restore.Spec.MariaDBRef, restore.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &restore, r.ConditionComplete.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	// We cannot check if mariaDb.IsReady() here and update the status accordingly
	// because we would be creating a deadlock when bootstrapping from backup

	backup, err := r.RefResolver.GetBackupMariaDB(ctx, restore.Spec.BackupRef, restore.Namespace)
	if err != nil {
		var backupErr *multierror.Error
		backupErr = multierror.Append(backupErr, err)

		err = r.patchStatus(ctx, &restore, r.ConditionComplete.RefResolverPatcher(err, backup))
		backupErr = multierror.Append(backupErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", backupErr)
	}

	if !backup.IsComplete() {
		if err := r.patchStatus(ctx, &restore, r.ConditionComplete.FailedPatcher("BackupMariaDB not complete")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching RestoreMariaDB: %v", err)
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	var jobErr *multierror.Error
	err = r.reconcileJob(ctx, &restore, mariaDb, backup, req.NamespacedName)
	jobErr = multierror.Append(jobErr, err)

	patcher, err := r.ConditionComplete.PatcherWithJob(ctx, err, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for RestoreMariaDB: %v", err)
	}

	err = r.patchStatus(ctx, &restore, patcher)
	jobErr = multierror.Append(jobErr, err)

	if err := jobErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *RestoreMariaDBReconciler) reconcileJob(ctx context.Context, restore *databasev1alpha1.RestoreMariaDB,
	mariaDb *databasev1alpha1.MariaDB, backup *databasev1alpha1.BackupMariaDB, key types.NamespacedName) error {

	desiredJob, err := r.Builder.BuildRestoreJob(restore, mariaDb, backup, key)
	if err != nil {
		return fmt.Errorf("error building restore Job: %v", err)
	}

	if err := r.JobReconciler.Reconcile(ctx, key, desiredJob); err != nil {
		return fmt.Errorf("error reconciling Job: %v", err)
	}
	return nil
}

func (r *RestoreMariaDBReconciler) patchStatus(ctx context.Context, restore *databasev1alpha1.RestoreMariaDB,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(restore.DeepCopy())
	patcher(&restore.Status)

	if err := r.Client.Status().Patch(ctx, restore, patch); err != nil {
		return fmt.Errorf("error patching BackupMariaDB status: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.RestoreMariaDB{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
