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
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SqlJobReconciler reconciles a SqlJob object
type SqlJobReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Builder             *builder.Builder
	RefResolver         *refresolver.RefResolver
	ConditionComplete   *conditions.Complete
	ConfigMapReconciler *configmap.ConfigMapReconciler
}

//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=sqljobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=sqljobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariadb.mmontes.io,resources=sqljobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SqlJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var sqlJob mariadbv1alpha1.SqlJob
	if err := r.Get(ctx, req.NamespacedName, &sqlJob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ok, result, err := r.waitForDependencies(ctx, &sqlJob)
	if !ok {
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error waiting for dependencies: %v", err)
		}
		return result, nil
	}

	mariaDb, err := r.RefResolver.MariaDB(ctx, &sqlJob.Spec.MariaDBRef, sqlJob.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &sqlJob, r.ConditionComplete.RefResolverPatcher(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if sqlJob.Spec.MariaDBRef.WaitForIt && !mariaDb.IsReady() {
		if err := r.patchStatus(ctx, &sqlJob, r.ConditionComplete.FailedPatcher("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching SqlJob: %v", err)
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	if err := r.reconcileConfigMap(ctx, &sqlJob); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling ConfigMap: %v", err)
	}

	var jobErr *multierror.Error
	err = r.reconcileJob(ctx, &sqlJob, mariaDb, req.NamespacedName)
	jobErr = multierror.Append(jobErr, err)

	patcher, err := r.ConditionComplete.PatcherWithJob(ctx, err, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for SqlJob: %v", err)
	}

	err = r.patchStatus(ctx, &sqlJob, patcher)
	jobErr = multierror.Append(jobErr, err)

	if err := jobErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating Job: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *SqlJobReconciler) waitForDependencies(ctx context.Context, sqlJob *v1alpha1.SqlJob) (bool, ctrl.Result, error) {
	if sqlJob.Spec.DependsOn == nil {
		return true, ctrl.Result{}, nil
	}
	requeueResult := ctrl.Result{RequeueAfter: 3 * time.Second}
	patchStatus := func(msg string) (bool, ctrl.Result, error) {
		if err := r.patchStatus(ctx, sqlJob, r.ConditionComplete.FailedPatcher(msg)); err != nil {
			return false, ctrl.Result{}, err
		}
		return false, requeueResult, nil
	}

	for _, dep := range sqlJob.Spec.DependsOn {
		sqlJobDep, err := r.RefResolver.SqlJob(ctx, &dep, sqlJob.Namespace)

		if err != nil {
			if apierrors.IsNotFound(err) {
				return patchStatus(fmt.Sprintf("Dependency '%s' not found", dep.Name))
			}
			return false, ctrl.Result{}, fmt.Errorf("error getting SqlJob dependency: %v", err)
		}
		if !sqlJobDep.IsComplete() {
			return patchStatus(fmt.Sprintf("Dependency '%s' not ready", dep.Name))
		}
	}
	return true, ctrl.Result{}, nil
}

func (r *SqlJobReconciler) reconcileConfigMap(ctx context.Context, sqlJob *mariadbv1alpha1.SqlJob) error {

	if r.ConfigMapReconciler.NoopReconcile(sqlJob) {
		return nil
	}

	key := configMapSqlJobKey(sqlJob)
	if err := r.ConfigMapReconciler.Reconcile(ctx, sqlJob, key); err != nil {
		return fmt.Errorf("error reconciling ConfigMap: %v", err)
	}

	if err := r.patch(ctx, sqlJob, func(sqlJob *mariadbv1alpha1.SqlJob) {
		sqlJob.Spec.SqlConfigMapKeyRef = &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: key.Name,
			},
			Key: r.ConfigMapReconciler.ConfigMapKey,
		}
	}); err != nil {
		return fmt.Errorf("error patching SqlJob: %v", err)
	}
	return nil
}

func (r *SqlJobReconciler) reconcileJob(ctx context.Context, sqlJob *mariadbv1alpha1.SqlJob,
	mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) error {

	desiredJob, err := r.Builder.BuildSqlJob(key, sqlJob, mariadb)
	if err != nil {
		return fmt.Errorf("error building Job: %v", err)
	}

	var existingJob batchv1.Job
	if err := r.Get(ctx, key, &existingJob); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Job: %v", err)
		}

		if err := r.Create(ctx, desiredJob); err != nil {
			return fmt.Errorf("error creating Job: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingJob.DeepCopy())
	existingJob.Spec.BackoffLimit = desiredJob.Spec.BackoffLimit

	if err := r.Patch(ctx, &existingJob, patch); err != nil {
		return fmt.Errorf("error patching Job: %v", err)
	}
	return nil
}

func (r *SqlJobReconciler) patchStatus(ctx context.Context, sqlJob *mariadbv1alpha1.SqlJob,
	patcher conditions.Patcher) error {
	patch := client.MergeFrom(sqlJob.DeepCopy())
	patcher(&sqlJob.Status)

	if err := r.Client.Status().Patch(ctx, sqlJob, patch); err != nil {
		return fmt.Errorf("error patching SqlJob status: %v", err)
	}
	return nil
}

func (r *SqlJobReconciler) patch(ctx context.Context, sqlJob *mariadbv1alpha1.SqlJob,
	patcher func(*mariadbv1alpha1.SqlJob)) error {
	patch := client.MergeFrom(sqlJob.DeepCopy())
	patcher(sqlJob)

	if err := r.Client.Patch(ctx, sqlJob, patch); err != nil {
		return fmt.Errorf("error patching SqlJob: %v", err)
	}
	return nil
}

func configMapSqlJobKey(sqlJob *mariadbv1alpha1.SqlJob) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("sql-%s", sqlJob.Name),
		Namespace: sqlJob.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SqlJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.SqlJob{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
