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
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ConditionComplete *conditions.Complete
	BatchReconciler   *batch.BatchReconciler
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=backups/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var backup mariadbv1alpha1.Backup
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariaDb, err := r.RefResolver.MariaDB(ctx, &backup.Spec.MariaDBRef, backup.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &backup, r.ConditionComplete.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if backup.Spec.MariaDBRef.WaitForIt && !mariaDb.IsReady() {
		if err := r.patchStatus(ctx, &backup, r.ConditionComplete.FailedPatcher("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Backup: %v", err)
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	var batchErr *multierror.Error
	err = r.BatchReconciler.Reconcile(ctx, &backup, mariaDb)
	batchErr = multierror.Append(batchErr, err)

	patcher, err := r.patcher(ctx, err, req.NamespacedName, &backup)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for Backup: %v", err)
	}

	err = r.patchStatus(ctx, &backup, patcher)
	batchErr = multierror.Append(batchErr, err)

	if err := batchErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *BackupReconciler) patcher(ctx context.Context, err error,
	key types.NamespacedName, backup *mariadbv1alpha1.Backup) (conditions.Patcher, error) {

	if backup.Spec.Schedule != nil {
		return r.ConditionComplete.PatcherWithCronJob(ctx, err, key)
	}
	return r.ConditionComplete.PatcherWithJob(ctx, err, key)
}

func (r *BackupReconciler) patchStatus(ctx context.Context, backup *mariadbv1alpha1.Backup,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(&backup.Status)

	if err := r.Client.Status().Patch(ctx, backup, patch); err != nil {
		return fmt.Errorf("error patching Backup status: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.Backup{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
