package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/v26/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensureSQLThread starts the replica SQL threads once per finished PhysicalBackup Job. It returns whether finished Jobs are still
// pending, so the caller requeues until they are processed.
// mariadb-backup --safe-slave-backup stops the SQL thread of the target replica and only starts it again if it was running when
// that attempt started. When the Job retries an attempt that died after stopping it, the replica is left without a running
// SQL thread and without an error, so nothing else recovers it.
func (r *PhysicalBackupReconciler) ensureSQLThread(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (pending bool, err error) {
	if !mariadb.IsReplicationEnabled() || backup.Spec.Storage.VolumeSnapshot != nil {
		return false, nil
	}
	jobs, err := r.sqlThreadPendingJobs(ctx, backup)
	if err != nil {
		return false, fmt.Errorf("error getting pending jobs: %v", err)
	}
	if len(jobs) == 0 {
		return false, nil
	}
	ensured, err := r.ensureReplicaSQLThreads(ctx, backup, mariadb, logger)
	if err != nil {
		// The SQL thread has not started yet; jobs are still pending.
		return true, fmt.Errorf("error ensuring replica SQL threads: %v", err)
	}
	if !ensured {
		// Expected wait (a PhysicalBackup Job is running or the MariaDB is not stable): jobs are still pending, not an error.
		return true, nil
	}

	for _, job := range jobs {
		patch := client.MergeFrom(job.DeepCopy())
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}
		job.Annotations[metadata.PhysicalBackupSQLThreadStartedAnnotation] = "true"
		if err := r.Patch(ctx, job, patch); err != nil {
			return true, fmt.Errorf("error patching Job '%s': %v", job.Name, err)
		}
	}
	return false, nil
}

// ensureReplicaSQLThreads starts the stopped replica SQL threads, unless the MariaDB is not in a stable state or a PhysicalBackup
// Job is running. It returns whether the SQL threads were ensured.
func (r *PhysicalBackupReconciler) ensureReplicaSQLThreads(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (bool, error) {
	if !mariadb.IsReplicationEnabled() || backup.Spec.Storage.VolumeSnapshot != nil {
		return false, nil
	}
	if mariadb.Status.CurrentPrimaryPodIndex == nil || !shouldReconcilePhysicalBackup(mariadb, logger.V(1)) {
		logger.V(1).Info("MariaDB not in a stable state. Skipping SQL thread check...")
		return false, nil
	}
	// Starting the SQL thread while any PhysicalBackup Job is running would break the consistency of that backup.
	hasRunningJobs, err := r.hasRunningPhysicalBackupJobs(ctx, mariadb)
	if err != nil {
		return false, fmt.Errorf("error checking running Jobs: %v", err)
	}
	if hasRunningJobs {
		logger.V(1).Info("PhysicalBackup Jobs running. Skipping SQL thread check...")
		return false, nil
	}

	if err := r.startReplicaSQLThreads(ctx, backup, mariadb, logger); err != nil {
		return false, fmt.Errorf("error starting replica SQL threads: %v", err)
	}
	return true, nil
}

// sqlThreadPendingJobs returns the finished Jobs whose replica SQL threads have not been ensured yet.
func (r *PhysicalBackupReconciler) sqlThreadPendingJobs(ctx context.Context,
	backup *mariadbv1alpha1.PhysicalBackup) ([]*batchv1.Job, error) {
	jobList, err := jobpkg.ListJobs(ctx, r.Client, backup)
	if err != nil {
		return nil, fmt.Errorf("error listing Jobs: %v", err)
	}

	var jobs []*batchv1.Job
	for i := range jobList.Items {
		job := &jobList.Items[i]
		if !jobpkg.IsJobComplete(job) && !jobpkg.IsJobFailed(job) {
			continue
		}
		if _, ok := job.Annotations[metadata.PhysicalBackupSQLThreadStartedAnnotation]; ok {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *PhysicalBackupReconciler) hasRunningPhysicalBackupJobs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	// Both lists are served from the informer cache, and only PhysicalBackups in the MariaDB namespace are considered:
	// PhysicalBackups can only refer to a MariaDB in the same namespace, as 'storage' PVCs need to be mounted.
	var backupList mariadbv1alpha1.PhysicalBackupList
	if err := r.List(ctx, &backupList, client.InNamespace(mariadb.Namespace)); err != nil {
		return false, fmt.Errorf("error listing PhysicalBackups: %v", err)
	}
	for i := range backupList.Items {
		backup := &backupList.Items[i]
		ref := backup.Spec.MariaDBRef
		refNamespace := ref.Namespace
		if refNamespace == "" {
			refNamespace = backup.Namespace
		}
		if ref.Name != mariadb.Name || refNamespace != mariadb.Namespace {
			continue
		}

		jobList, err := jobpkg.ListJobs(ctx, r.Client, backup)
		if err != nil {
			return false, fmt.Errorf("error listing Jobs: %v", err)
		}
		for j := range jobList.Items {
			if jobpkg.IsJobRunning(&jobList.Items[j]) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *PhysicalBackupReconciler) startReplicaSQLThreads(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	pods, err := mariadbpod.ListMariaDBSecondaryPods(ctx, r.Client, mariadb)
	if err != nil {
		return fmt.Errorf("error listing Pods: %v", err)
	}

	var errBundle *multierror.Error
	for i := range pods {
		pod := &pods[i]
		podIndex, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			errBundle = multierror.Append(errBundle, fmt.Errorf("error getting index for Pod '%s': %v", pod.Name, err))
			continue
		}
		started, err := r.startReplicaSQLThread(ctx, mariadb, *podIndex, logger.WithValues("replica", pod.Name))
		if err != nil {
			errBundle = multierror.Append(errBundle, fmt.Errorf("error starting SQL thread in Pod '%s': %v", pod.Name, err))
			continue
		}
		if started {
			r.Recorder.Eventf(
				backup,
				mariadb,
				corev1.EventTypeNormal,
				mariadbv1alpha1.ReasonReplicaSQLThreadStarted,
				mariadbv1alpha1.ReasonReplicaSQLThreadStarted,
				"Replica SQL thread started in Pod %s",
				pod.Name,
			)
		}
	}
	return errBundle.ErrorOrNil()
}

func (r *PhysicalBackupReconciler) startReplicaSQLThread(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, podIndex int,
	logger logr.Logger) (bool, error) {
	sqlClient, err := sql.NewInternalClientWithPodIndex(ctx, mariadb, r.RefResolver, podIndex)
	if err != nil {
		return false, fmt.Errorf("error getting SQL client: %v", err)
	}
	defer sqlClient.Close()

	isReplica, err := sqlClient.IsReplicationReplica(ctx)
	if err != nil {
		return false, fmt.Errorf("error checking replica: %v", err)
	}
	if !isReplica {
		return false, nil
	}
	status, err := sqlClient.ReplicaStatus(ctx, logger)
	if err != nil {
		return false, fmt.Errorf("error getting replica status: %v", err)
	}
	if ptr.Deref(status.SlaveSQLRunning, false) {
		return false, nil
	}
	// A stop with an error is a replication failure, which must remain visible.
	if lastSQLErrno := ptr.Deref(status.LastSQLErrno, 0); lastSQLErrno != 0 {
		logger.Info("Replica SQL thread stopped with error. Skipping...", "Last_SQL_Errno", lastSQLErrno)
		return false, nil
	}

	logger.Info("Starting replica SQL thread")
	if err := sqlClient.StartSlaveSQLThread(ctx); err != nil {
		return false, err
	}
	return true, nil
}
