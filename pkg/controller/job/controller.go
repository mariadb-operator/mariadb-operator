package job

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

type JobReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	builder     *builder.Builder
}

func NewJobReconciler(client client.Client, refResolver *refresolver.RefResolver, builder *builder.Builder) *JobReconciler {
	return &JobReconciler{
		Client:      client,
		refResolver: refResolver,
		builder:     builder,
	}
}

func (r *JobReconciler) Reconcile(ctx context.Context, parentObj client.Object,
	mariaDB *databasev1alpha1.MariaDB) error {

	key := types.NamespacedName{
		Name:      parentObj.GetName(),
		Namespace: parentObj.GetNamespace(),
	}

	if err := r.reconcileStorage(ctx, key, parentObj); err != nil {
		return fmt.Errorf("error reconciling storage: %v", err)
	}

	desiredJob, err := r.buildJob(ctx, key, parentObj, mariaDB)
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

func (r *JobReconciler) reconcileStorage(ctx context.Context, key types.NamespacedName,
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

func (r *JobReconciler) buildJob(ctx context.Context, key types.NamespacedName, parentObj client.Object,
	mariaDB *databasev1alpha1.MariaDB) (*batchv1.Job, error) {

	if backup, ok := parentObj.(*databasev1alpha1.BackupMariaDB); ok {
		return r.builder.BuildBackupJob(key, backup, mariaDB)
	}

	if restore, ok := parentObj.(*databasev1alpha1.RestoreMariaDB); ok {
		backup, err := r.refResolver.GetBackupMariaDB(ctx, restore.Spec.BackupRef, key.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting BackupMariaDB: %v", err)
		}
		return r.builder.BuildRestoreJob(key, restore, backup, mariaDB)
	}

	return nil, fmt.Errorf("unsupported parent object type: '%T'", parentObj)
}
