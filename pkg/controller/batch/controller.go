package batch

import (
	"context"
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BatchReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	builder     *builder.Builder
}

func NewBatchReconciler(client client.Client, refResolver *refresolver.RefResolver, builder *builder.Builder) *BatchReconciler {
	return &BatchReconciler{
		Client:      client,
		refResolver: refResolver,
		builder:     builder,
	}
}

func (r *BatchReconciler) Reconcile(ctx context.Context, parentObj client.Object,
	mariaDB *databasev1alpha1.MariaDB) error {

	key := types.NamespacedName{
		Name:      parentObj.GetName(),
		Namespace: parentObj.GetNamespace(),
	}
	if err := r.reconcileStorage(ctx, key, parentObj); err != nil {
		return fmt.Errorf("error reconciling storage: %v", err)
	}
	if err := r.reconcileBatch(ctx, key, parentObj, mariaDB); err != nil {
		return fmt.Errorf("error reconciling batch: %v", err)
	}
	return nil
}

func (r *BatchReconciler) reconcileStorage(ctx context.Context, key types.NamespacedName,
	parentObj client.Object) error {

	backup, ok := parentObj.(*databasev1alpha1.BackupMariaDB)
	if !ok {
		return nil
	}
	if ok && backup.Spec.Storage.PersistentVolumeClaim == nil {
		return nil
	}

	var existingPvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, key, &existingPvc)
	if err == nil {
		return nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error creating PersistentVolumeClaim: %v", err)
	}

	pvcMeta := metav1.ObjectMeta{
		Name:      parentObj.GetName(),
		Namespace: parentObj.GetNamespace(),
	}
	pvc := r.builder.BuildPVC(pvcMeta, &backup.Spec.Storage)

	if err := r.Create(ctx, pvc); err != nil {
		return fmt.Errorf("error creating PersistentVolumeClain: %v", err)
	}
	return nil
}

func (r *BatchReconciler) reconcileBatch(ctx context.Context, key types.NamespacedName,
	parentObj client.Object, mariaDB *databasev1alpha1.MariaDB) error {

	desiredBatch, err := r.buildBatch(ctx, key, parentObj, mariaDB)
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

func (r *BatchReconciler) buildBatch(ctx context.Context, key types.NamespacedName, parentObj client.Object,
	mariaDB *databasev1alpha1.MariaDB) (client.Object, error) {

	if backup, ok := parentObj.(*databasev1alpha1.BackupMariaDB); ok {
		if backup.Spec.Schedule != nil {
			return r.builder.BuildBackupCronJob(key, backup, mariaDB)
		}
		return r.builder.BuildBackupJob(key, backup, mariaDB)
	}

	if restore, ok := parentObj.(*databasev1alpha1.RestoreMariaDB); ok {
		backup, err := r.refResolver.GetBackupMariaDB(ctx, &restore.Spec.BackupRef, key.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting BackupMariaDB: %v", err)
		}
		return r.builder.BuildRestoreJob(key, restore, backup, mariaDB, restore.Spec.BackupRef.FileName)
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
	existingCronJob.Spec.Schedule = desiredCronJob.Spec.Schedule
	existingCronJob.Spec.Suspend = desiredCronJob.Spec.Suspend
	existingCronJob.Spec.JobTemplate.Spec.BackoffLimit = desiredCronJob.Spec.JobTemplate.Spec.BackoffLimit

	if err := r.Patch(ctx, &existingCronJob, patch); err != nil {
		return fmt.Errorf("error patching CronJob: %v", err)
	}
	return nil
}
