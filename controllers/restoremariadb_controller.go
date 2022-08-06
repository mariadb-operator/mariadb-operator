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
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
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
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=restoremariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RestoreMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var restore databasev1alpha1.RestoreMariaDB
	if err := r.Get(ctx, req.NamespacedName, &restore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var jobErr *multierror.Error
	err := r.createJob(ctx, &restore, req.NamespacedName)
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

	job, err := r.Builder.BuildRestoreJob(restore, mariadb, backup, key)
	if err != nil {
		return fmt.Errorf("error building restore Job: %v", err)
	}

	if err := r.Create(ctx, job); err != nil {
		return fmt.Errorf("error creating Job: %v", err)
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
