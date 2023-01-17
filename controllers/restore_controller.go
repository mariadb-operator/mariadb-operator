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
	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/controller/batch"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errBackupNotComplete = errors.New("Backup not complete")
)

// RestoreReconciler reconciles a restore object
type RestoreReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ConditionComplete *conditions.Complete
	BatchReconciler   *batch.BatchReconciler
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=restores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=restores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var restore mariadbv1alpha1.Restore
	if err := r.Get(ctx, req.NamespacedName, &restore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariaDb, err := r.RefResolver.MariaDB(ctx, &restore.Spec.MariaDBRef, restore.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &restore, r.ConditionComplete.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	// We cannot check if mariaDb.IsReady() here and update the status accordingly
	// because we would be creating a deadlock when bootstrapping from backup
	// TODO: add a IsBootstrapping() method to MariaDB?

	if err := r.initSource(ctx, &restore); err != nil {
		if errors.Is(err, errBackupNotComplete) {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error initializing source: %v", err)
	}

	var jobErr *multierror.Error
	err = r.BatchReconciler.Reconcile(ctx, &restore, mariaDb)
	jobErr = multierror.Append(jobErr, err)

	patcher, err := r.ConditionComplete.PatcherWithJob(ctx, err, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for restore: %v", err)
	}

	err = r.patchStatus(ctx, &restore, patcher)
	jobErr = multierror.Append(jobErr, err)

	if err := jobErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *RestoreReconciler) initSource(ctx context.Context, restore *mariadbv1alpha1.Restore) error {
	if restore.Spec.RestoreSource.IsInit() {
		return nil
	}
	if restore.Spec.RestoreSource.BackupRef == nil {
		var restoreErr *multierror.Error
		restoreErr = multierror.Append(restoreErr, errors.New("unable to determine restore source, 'backupRef' is nil"))

		err := r.patchStatus(ctx, restore, r.ConditionComplete.FailedPatcher("Unable to determine restore source"))
		restoreErr = multierror.Append(restoreErr, err)

		return restoreErr
	}

	backup, err := r.RefResolver.Backup(ctx, restore.Spec.RestoreSource.BackupRef, restore.Namespace)
	if err != nil {
		var backupErr *multierror.Error
		backupErr = multierror.Append(backupErr, err)

		err = r.patchStatus(ctx, restore, r.ConditionComplete.RefResolverPatcher(err, backup))
		backupErr = multierror.Append(backupErr, err)

		return fmt.Errorf("error getting Backup: %v", backupErr)
	}

	if !backup.IsComplete() {
		if err := r.patchStatus(ctx, restore, r.ConditionComplete.FailedPatcher("Backup not complete")); err != nil {
			return fmt.Errorf("error patching restore: %v", err)
		}
		return errBackupNotComplete
	}

	patcher := func(r *mariadbv1alpha1.Restore) {
		r.Spec.RestoreSource.Init(backup)
	}
	if err := r.patch(ctx, restore, patcher); err != nil {
		return fmt.Errorf("error patching restore: %v", err)
	}
	return nil
}

func (r *RestoreReconciler) patchStatus(ctx context.Context, restore *mariadbv1alpha1.Restore,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(restore.DeepCopy())
	patcher(&restore.Status)

	if err := r.Client.Status().Patch(ctx, restore, patch); err != nil {
		return fmt.Errorf("error patching restore status: %v", err)
	}
	return nil
}

func (r *RestoreReconciler) patch(ctx context.Context, restore *mariadbv1alpha1.Restore,
	patcher func(*mariadbv1alpha1.Restore)) error {
	patch := client.MergeFrom(restore.DeepCopy())
	patcher(restore)

	if err := r.Client.Patch(ctx, restore, patch); err != nil {
		return fmt.Errorf("error patching restore: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.Restore{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
