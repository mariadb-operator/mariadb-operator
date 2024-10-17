package batch

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BatchReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	builder     *builder.Builder
}

func NewBatchReconciler(client client.Client, builder *builder.Builder) *BatchReconciler {
	return &BatchReconciler{
		Client:      client,
		refResolver: refresolver.New(client),
		builder:     builder,
	}
}

func (r *BatchReconciler) Reconcile(ctx context.Context, parentObj client.Object,
	mariadb *mariadbv1alpha1.MariaDB) error {
	if err := r.reconcileStorage(ctx, parentObj); err != nil {
		return fmt.Errorf("error reconciling storage: %v", err)
	}
	if err := r.reconcileBatch(ctx, parentObj, mariadb); err != nil {
		return fmt.Errorf("error reconciling batch: %v", err)
	}
	return nil
}

func (r *BatchReconciler) reconcileStorage(ctx context.Context, parentObj client.Object) error {
	if backup, ok := parentObj.(*mariadbv1alpha1.Backup); ok {
		if backup.Spec.Storage.PersistentVolumeClaim != nil {
			pvc, err := r.builder.BuildBackupStoragePVC(backup.StoragePVCKey(), backup)
			if err != nil {
				return fmt.Errorf("error building Backup storage PVC: %v", err)
			}

			if err := r.createPVC(ctx, pvc); err != nil {
				return fmt.Errorf("error creating Backup storage PVC: %v", err)
			}
		}

		stagingStorage := ptr.Deref(backup.Spec.StagingStorage, mariadbv1alpha1.BackupStagingStorage{})
		if stagingStorage.PersistentVolumeClaim != nil {
			pvc, err := r.builder.BuildBackupStagingPVC(
				backup.StagingPVCKey(),
				stagingStorage.PersistentVolumeClaim,
				backup.Spec.InheritMetadata,
				backup,
			)
			if err != nil {
				return fmt.Errorf("error building Backup staging PVC: %v", err)
			}

			if err := r.createPVC(ctx, pvc); err != nil {
				return fmt.Errorf("error creating Backup staging PVC: %v", err)
			}
		}
	}

	if restore, ok := parentObj.(*mariadbv1alpha1.Restore); ok {
		stagingStorage := ptr.Deref(restore.Spec.StagingStorage, mariadbv1alpha1.BackupStagingStorage{})
		if stagingStorage.PersistentVolumeClaim != nil {
			pvc, err := r.builder.BuildBackupStagingPVC(
				restore.StagingPVCKey(),
				stagingStorage.PersistentVolumeClaim,
				restore.Spec.InheritMetadata,
				restore,
			)
			if err != nil {
				return fmt.Errorf("error building Restore staging PVC: %v", err)
			}

			if err := r.createPVC(ctx, pvc); err != nil {
				return fmt.Errorf("error creating Restore staging PVC: %v", err)
			}
		}
	}

	return nil
}

func (r *BatchReconciler) createPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	key := client.ObjectKeyFromObject(pvc)
	var existingPvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, key, &existingPvc)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error creating PersistentVolumeClaim: %v", err)
	}
	return r.Create(ctx, pvc)
}

func (r *BatchReconciler) reconcileBatch(ctx context.Context, parentObj client.Object, mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(parentObj)
	desiredBatch, err := r.buildBatch(parentObj, mariadb)
	if err != nil {
		return fmt.Errorf("error building Job: %v", err)
	}

	if desiredJob, ok := desiredBatch.(*batchv1.Job); ok {
		return r.reconcileJob(ctx, key, desiredJob)
	}
	if desiredCronJob, ok := desiredBatch.(*batchv1.CronJob); ok {
		return r.reconcileCronJob(ctx, key, desiredCronJob)
	}

	return fmt.Errorf("unable to reconcile batch object using type: '%T'", parentObj)
}

func (r *BatchReconciler) buildBatch(parentObj client.Object, mariadb *mariadbv1alpha1.MariaDB) (client.Object, error) {
	key := client.ObjectKeyFromObject(parentObj)
	if backup, ok := parentObj.(*mariadbv1alpha1.Backup); ok {
		if backup.Spec.Schedule != nil {
			return r.builder.BuildBackupCronJob(key, backup, mariadb)
		}
		return r.builder.BuildBackupJob(key, backup, mariadb)
	}

	if restore, ok := parentObj.(*mariadbv1alpha1.Restore); ok {
		return r.builder.BuildRestoreJob(key, restore, mariadb)
	}

	return nil, fmt.Errorf("unable to build batch object using type: '%T'", parentObj)
}

func (r *BatchReconciler) reconcileJob(ctx context.Context, key types.NamespacedName,
	desiredJob *batchv1.Job) error {

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

func (r *BatchReconciler) reconcileCronJob(ctx context.Context, key types.NamespacedName,
	desiredCronJob *batchv1.CronJob) error {

	var existingCronJob batchv1.CronJob
	if err := r.Get(ctx, key, &existingCronJob); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting CronJob: %v", err)
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
