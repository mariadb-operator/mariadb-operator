package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	jobConfigMapKey = "job.sql"
)

// SqlJobReconciler reconciles a SqlJob object
type SQLJobReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	Builder             *builder.Builder
	RefResolver         *refresolver.RefResolver
	ConditionComplete   *condition.Complete
	ConfigMapReconciler *configmap.ConfigMapReconciler
	RBACReconciler      *rbac.RBACReconciler
	RequeueInterval     time.Duration
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=sqljobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=sqljobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=sqljobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SQLJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var sqlJob mariadbv1alpha1.SQLJob
	if err := r.Get(ctx, req.NamespacedName, &sqlJob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ok, result, err := r.waitForDependencies(ctx, &sqlJob)
	if !ok {
		return result, err
	}

	mariadb, err := r.RefResolver.MariaDB(ctx, &sqlJob.Spec.MariaDBRef, sqlJob.Namespace)
	if err != nil {
		var mariaDBErr *multierror.Error
		mariaDBErr = multierror.Append(mariaDBErr, err)

		err = r.patchStatus(ctx, &sqlJob, r.ConditionComplete.PatcherRefResolver(err, mariadb))
		mariaDBErr = multierror.Append(mariaDBErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDBErr)
	}

	if sqlJob.Spec.MariaDBRef.WaitForIt && !mariadb.IsReady() {
		if err := r.patchStatus(ctx, &sqlJob, r.ConditionComplete.PatcherFailed("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching SqlJob: %v", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.setDefaults(ctx, &sqlJob, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting SqlJob: %v", err)
	}

	if err := r.reconcileServiceAccount(ctx, &sqlJob); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling ServiceAccount: %v", err)
	}
	if err := r.reconcileConfigMap(ctx, &sqlJob); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling ConfigMap: %v", err)
	}

	var jobErr *multierror.Error
	err = r.reconcileBatch(ctx, &sqlJob, mariadb, req.NamespacedName)
	jobErr = multierror.Append(jobErr, err)

	patcher, err := r.patcher(ctx, &sqlJob, err, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, fmt.Errorf("error getting patcher for SqlJob: %v", err)
	}

	err = r.patchStatus(ctx, &sqlJob, patcher)
	jobErr = multierror.Append(jobErr, err)

	if err := jobErr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling SqlJob: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *SQLJobReconciler) waitForDependencies(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob) (bool, ctrl.Result, error) {
	if sqlJob.Spec.DependsOn == nil {
		return true, ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx)

	for _, dep := range sqlJob.Spec.DependsOn {
		sqlJobDep, err := r.RefResolver.SQLJob(ctx, &dep, sqlJob.Namespace)

		if err != nil {
			msg := fmt.Sprintf("Error getting SqlJob dependency: %v", err)
			if apierrors.IsNotFound(err) {
				msg = fmt.Sprintf("Dependency '%s' not found", dep.Name)
			}

			logger.Info(msg)
			return false, ctrl.Result{RequeueAfter: r.RequeueInterval}, r.patchStatus(ctx, sqlJob, r.ConditionComplete.PatcherFailed(msg))
		}
		if !sqlJobDep.IsComplete() {
			msg := fmt.Sprintf("Dependency '%s' not ready", dep.Name)

			logger.Info(msg)
			if err := r.patchStatus(ctx, sqlJob, r.ConditionComplete.PatcherFailed(msg)); err != nil {
				return false, ctrl.Result{}, err
			}
			return false, ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
	}
	return true, ctrl.Result{}, nil
}

func (r *SQLJobReconciler) reconcileConfigMap(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob) error {
	key := configMapSQLJobKey(sqlJob)
	if sqlJob.Spec.SQL != nil && sqlJob.Spec.SQLConfigMapKeyRef == nil {
		req := configmap.ReconcileRequest{
			Metadata: sqlJob.Spec.InheritMetadata,
			Owner:    sqlJob,
			Key:      key,
			Data: map[string]string{
				jobConfigMapKey: *sqlJob.Spec.SQL,
			},
		}
		if err := r.ConfigMapReconciler.Reconcile(ctx, &req); err != nil {
			return err
		}
	}
	if sqlJob.Spec.SQLConfigMapKeyRef != nil {
		return nil
	}

	return r.patch(ctx, sqlJob, func(sqlJob *mariadbv1alpha1.SQLJob) {
		sqlJob.Spec.SQLConfigMapKeyRef = &mariadbv1alpha1.ConfigMapKeySelector{
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: key.Name,
			},
			Key: jobConfigMapKey,
		}
	})
}

func (r *SQLJobReconciler) reconcileBatch(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob,
	mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) error {
	if sqlJob.Spec.Schedule != nil {
		return r.reconcileCronJob(ctx, sqlJob, mariadb, key)
	}
	return r.reconcileJob(ctx, sqlJob, mariadb, key)
}

func (r *SQLJobReconciler) patcher(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob, err error,
	key types.NamespacedName) (condition.Patcher, error) {
	if sqlJob.Spec.Schedule != nil {
		return r.ConditionComplete.PatcherWithCronJob(ctx, err, key)
	}
	return r.ConditionComplete.PatcherWithJob(ctx, err, key)
}

func (r *SQLJobReconciler) reconcileJob(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob,
	mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) error {
	desiredJob, err := r.Builder.BuildSQLJob(key, sqlJob, mariadb)
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

func (r *SQLJobReconciler) reconcileCronJob(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob,
	mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) error {
	desiredCronJob, err := r.Builder.BuildSQLCronJob(key, sqlJob, mariadb)
	if err != nil {
		return fmt.Errorf("error building CronJob: %v", err)
	}

	var existingCronJob batchv1.CronJob
	if err := r.Get(ctx, key, &existingCronJob); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Job: %v", err)
		}

		if err := r.Create(ctx, desiredCronJob); err != nil {
			return fmt.Errorf("error creating CronJob: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingCronJob.DeepCopy())
	existingCronJob.Spec.FailedJobsHistoryLimit = desiredCronJob.Spec.FailedJobsHistoryLimit
	existingCronJob.Spec.SuccessfulJobsHistoryLimit = desiredCronJob.Spec.SuccessfulJobsHistoryLimit
	existingCronJob.Spec.TimeZone = desiredCronJob.Spec.TimeZone
	existingCronJob.Spec.Schedule = desiredCronJob.Spec.Schedule
	existingCronJob.Spec.Suspend = desiredCronJob.Spec.Suspend
	existingCronJob.Spec.JobTemplate.Spec.BackoffLimit = desiredCronJob.Spec.JobTemplate.Spec.BackoffLimit

	if err := r.Patch(ctx, &existingCronJob, patch); err != nil {
		return fmt.Errorf("error patching CronJob: %v", err)
	}
	return nil
}

func (r *SQLJobReconciler) setDefaults(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob,
	mariadb *mariadbv1alpha1.MariaDB) error {
	return r.patch(ctx, sqlJob, func(s *mariadbv1alpha1.SQLJob) {
		s.SetDefaults(mariadb)
	})
}

func (r *SQLJobReconciler) reconcileServiceAccount(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob) error {
	key := sqlJob.Spec.ServiceAccountKey(sqlJob.ObjectMeta)
	_, err := r.RBACReconciler.ReconcileServiceAccount(ctx, key, sqlJob, sqlJob.Spec.InheritMetadata)
	return err
}

func (r *SQLJobReconciler) patchStatus(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob,
	patcher condition.Patcher) error {
	patch := client.MergeFrom(sqlJob.DeepCopy())
	patcher(&sqlJob.Status)

	if err := r.Client.Status().Patch(ctx, sqlJob, patch); err != nil {
		return fmt.Errorf("error patching SqlJob status: %v", err)
	}
	return nil
}

func (r *SQLJobReconciler) patch(ctx context.Context, sqlJob *mariadbv1alpha1.SQLJob,
	patcher func(*mariadbv1alpha1.SQLJob)) error {
	patch := client.MergeFrom(sqlJob.DeepCopy())
	patcher(sqlJob)

	if err := r.Patch(ctx, sqlJob, patch); err != nil {
		return fmt.Errorf("error patching SqlJob: %v", err)
	}
	return nil
}

func configMapSQLJobKey(sqlJob *mariadbv1alpha1.SQLJob) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("sql-%s", sqlJob.Name),
		Namespace: sqlJob.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SQLJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.SQLJob{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
