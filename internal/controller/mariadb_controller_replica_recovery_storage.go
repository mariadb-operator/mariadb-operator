package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cleanupReplicaRecoveryBackupStorage removes the backup objects produced by a replica recovery from
// blob storage. Recovery backups are transient, but they are written to the same prefix and retention
// policy as the regular backups taken from the template, where nothing distinguishes them: a single
// recovery leaves a full copy of the dataset behind until the template retention expires it, which is
// enough to exhaust the backup storage on its own.
func (r *MariaDBReconciler) cleanupReplicaRecoveryBackupStorage(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.PhysicalBackupReplicaRecoveryKey()
	var backup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &backup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting replica recovery PhysicalBackup: %v", err)
	}
	if backup.Spec.Storage.S3 == nil && backup.Spec.Storage.AzureBlob == nil {
		return nil
	}

	jobList, err := jobpkg.ListJobs(ctx, r.Client, &backup)
	if err != nil {
		return fmt.Errorf("error listing replica recovery Jobs: %v", err)
	}
	fileNames, err := replicaRecoveryBackupFileNames(&backup, jobList)
	if err != nil {
		return fmt.Errorf("error getting replica recovery backup file names: %v", err)
	}
	if len(fileNames) == 0 {
		return nil
	}

	logger := log.FromContext(ctx).WithName("replica-recovery-storage")
	storageClient, err := r.getBlobStorageClient(ctx, backup.Spec.Storage.S3, backup.Spec.Storage.AzureBlob,
		backup.Namespace)
	if err != nil {
		r.recordReplicaRecoveryStorageLeak(mariadb, fileNames, err, logger)
		return nil
	}

	for _, fileName := range fileNames {
		if err := storageClient.RemoveWithOptions(ctx, fileName); err != nil && !storageClient.IsNotFound(err) {
			r.recordReplicaRecoveryStorageLeak(mariadb, []string{fileName}, err, logger)
			continue
		}
		logger.Info("Deleted replica recovery backup", "file", fileName)
	}
	return nil
}

// recordReplicaRecoveryStorageLeak reports objects that could not be deleted. Cleanup is best effort:
// blocking recovery completion on the availability of the backup storage would leave the cluster stuck
// in a recovering state, so a leaked object is surfaced instead of retried forever.
func (r *MariaDBReconciler) recordReplicaRecoveryStorageLeak(mariadb *mariadbv1alpha1.MariaDB,
	fileNames []string, err error, logger logr.Logger) {
	logger.Error(err, "Error deleting replica recovery backups", "files", fileNames)
	r.Recorder.Eventf(
		mariadb,
		nil,
		corev1.EventTypeWarning,
		mariadbv1alpha1.ReasonMariaDBReplicaRecoveryStorageLeak,
		mariadbv1alpha1.ActionReconciling,
		"Unable to delete replica recovery backups %v: %v",
		fileNames,
		err,
	)
}

// replicaRecoveryBackupFileNames derives the backup file names written by the recovery Jobs. Both the
// Job name and the backup file name are derived from the same scheduling instant, so the Jobs owned by
// the recovery PhysicalBackup identify exactly the objects it produced.
func replicaRecoveryBackupFileNames(backup *mariadbv1alpha1.PhysicalBackup,
	jobList *batchv1.JobList) ([]string, error) {
	if jobList == nil {
		return nil, nil
	}
	fileNames := make([]string, 0, len(jobList.Items))
	for _, job := range jobList.Items {
		jobTime, err := mariadbv1alpha1.ParsePhysicalBackupTime(job.Name)
		if err != nil {
			return nil, fmt.Errorf("error parsing Job time \"%s\": %v", job.Name, err)
		}
		fileName, err := getBackupFileName(backup, jobTime)
		if err != nil {
			return nil, fmt.Errorf("error getting backup file name: %v", err)
		}
		fileNames = append(fileNames, fileName)
	}
	return fileNames, nil
}
