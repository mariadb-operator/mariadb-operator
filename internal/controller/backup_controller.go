package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ConditionComplete *condition.Complete
	RBACReconciler    *rbac.RBACReconciler
	BatchReconciler   *batch.BatchReconciler
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=backups/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var backup mariadbv1alpha1.Backup
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if backup.Spec.Schedule == nil && backup.IsComplete() {
		if err := r.cleanupJobs(ctx, &backup); err != nil {
			return ctrl.Result{}, fmt.Errorf("error cleaning up Job: %v", err)
		}
		return ctrl.Result{}, nil
	}
	mariaDb, err := r.RefResolver.MariaDBObject(ctx, &backup.Spec.MariaDBRef, backup.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &backup, r.ConditionComplete.PatcherRefResolver(err, mariaDb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}

	if backup.Spec.MariaDBRef.WaitForIt && !mariaDb.IsReady() {
		if err := r.patchStatus(ctx, &backup, r.ConditionComplete.PatcherFailed("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Backup: %v", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.setDefaults(ctx, &backup, mariaDb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting Backup: %v", err)
	}

	if err := r.reconcileServiceAccount(ctx, &backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling ServiceAccount: %v", err)
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

func (r *BackupReconciler) setDefaults(ctx context.Context, backup *mariadbv1alpha1.Backup,
	mariadb interfaces.MariaDBObject) error {
	return r.patch(ctx, backup, func(b *mariadbv1alpha1.Backup) {
		if externalMariaDB, ok := mariadb.(*mariadbv1alpha1.ExternalMariaDB); ok {
			backup.SetExternalDefaults(externalMariaDB)
		} else if mariadbObj, ok := mariadb.(*mariadbv1alpha1.MariaDB); ok {
			backup.SetDefaults(mariadbObj)
		}
	})
}

func (r *BackupReconciler) reconcileServiceAccount(ctx context.Context, backup *mariadbv1alpha1.Backup) error {
	key := backup.Spec.ServiceAccountKey(backup.ObjectMeta)
	_, err := r.RBACReconciler.ReconcileServiceAccount(ctx, key, backup, backup.Spec.InheritMetadata)
	return err
}

func (r *BackupReconciler) patch(ctx context.Context, backup *mariadbv1alpha1.Backup, patcher func(*mariadbv1alpha1.Backup)) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(backup)
	return r.Patch(ctx, backup, patch)
}

func (r *BackupReconciler) patcher(ctx context.Context, err error,
	key types.NamespacedName, backup *mariadbv1alpha1.Backup) (condition.Patcher, error) {

	if backup.Spec.Schedule != nil {
		return r.ConditionComplete.PatcherWithCronJob(ctx, err, key)
	}
	return r.ConditionComplete.PatcherWithJob(ctx, err, key)
}

func (r *BackupReconciler) patchStatus(ctx context.Context, backup *mariadbv1alpha1.Backup,
	patcher condition.Patcher) error {
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
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *BackupReconciler) cleanupJobs(ctx context.Context, backup *mariadbv1alpha1.Backup) error {
	logger := log.FromContext(ctx).WithName("cleanup")
	var jobList batchv1.JobList
	if err := r.List(ctx, &jobList, client.InNamespace(backup.Namespace)); err != nil {
		return fmt.Errorf("error listing Jobs: %v", err)
	}

	var oneOffBackupJobs []batchv1.Job
	for _, job := range jobList.Items {
		owner := metav1.GetControllerOf(&job)
		if owner == nil || owner.Kind != "Backup" || owner.APIVersion != mariadbv1alpha1.GroupVersion.String() {
			continue
		}

		var parentBackup mariadbv1alpha1.Backup
		parentKey := types.NamespacedName{
			Name:      owner.Name,
			Namespace: job.Namespace,
		}
		if err := r.Get(ctx, parentKey, &parentBackup); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("error getting parent Backup: %v", err)
		}

		if parentBackup.Spec.Schedule == nil && parentBackup.Spec.MariaDBRef.Name == backup.Spec.MariaDBRef.Name {
			oneOffBackupJobs = append(oneOffBackupJobs, job)
		}
	}

	sort.Slice(oneOffBackupJobs, func(i, j int) bool {
		return oneOffBackupJobs[i].CreationTimestamp.Before(&oneOffBackupJobs[j].CreationTimestamp)
	})

	var completeJobs []*batchv1.Job
	var failedJobs []*batchv1.Job
	for i := range oneOffBackupJobs {
		job := &oneOffBackupJobs[i]
		if jobpkg.IsJobComplete(job) {
			completeJobs = append(completeJobs, job)
		} else if jobpkg.IsJobFailed(job) {
			failedJobs = append(failedJobs, job)
		}
	}

	maxHistory := int(ptr.Deref(backup.Spec.SuccessfulJobsHistoryLimit, 5))
	if len(completeJobs) > maxHistory {
		for i := 0; i < len(completeJobs)-maxHistory; i++ {
			logger.Info("Deleting completed one-off Backup Job", "job", completeJobs[i].Name)
			if err := r.Delete(ctx, completeJobs[i], &client.DeleteOptions{
				PropagationPolicy: ptr.To(metav1.DeletePropagationBackground),
			}); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}

	maxFailedHistory := int(ptr.Deref(backup.Spec.FailedJobsHistoryLimit, 5))
	if len(failedJobs) > maxFailedHistory {
		for i := 0; i < len(failedJobs)-maxFailedHistory; i++ {
			logger.Info("Deleting failed one-off Backup Job", "job", failedJobs[i].Name)
			if err := r.Delete(ctx, failedJobs[i], &client.DeleteOptions{
				PropagationPolicy: ptr.To(metav1.DeletePropagationBackground),
			}); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
	}

	return nil
}
