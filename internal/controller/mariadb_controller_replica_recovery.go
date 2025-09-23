package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/command"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	podobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/wait"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// These errors will trigger the recovery process straight away
var recoverableIOErrorCodes = []int{
	// Error 1236: Got fatal error from master when reading data from binary log.
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1200-to-1299/e1236
	1236,
	// Error 1945: Connecting slave requested to start from GTID, which is not in the master's binlog
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1900-to-1999/e1945
	1945,
	// Error 1947: Specified GTID conflicts with the binary log which contains a more recent GTID
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1900-to-1999/e1947
	1947,
	// Error 1951: The binlog on the master is missing the GTID requested by the slave
	// https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1900-to-1999/e1951
	1951,
	// Error 1955: Connecting slave requested to start from GTID which is not in the master's binlog
	// https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1900-to-1999/e1955
	1955,
}

var recoverableSQLErrorCodes = []int{
	// Error 1062: Duplicate entry for key.
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1062
	1062,
	// Error 1032: Can't find record in
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1032
	1032,
	// Error 1034: Incorrect key file for table; try to repair it
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1034
	1034,
	// Error 1049: Unknown database
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1049
	1049,
	// Error 1046: No database selected
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1046
	1146,
}

var externalUserPrivilegesSkippableSQLErrorCodes = []int{
	// Error 1133: Can't find any matching row in the user table
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1100-to-1199/e1133
	1133,
	// Error 1269: Can't revoke all privileges for one or more of the requested users
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1200-to-1299/e1269
	1269,
	// Error 1396: Operation failed for
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1300-to-1399/e1396
	1396,
}

// These errors will never trigger the recovery process
var notRecoverableIOErrorCodes = []int{
	// Error 2003: Can't connect to MariaDB server.
	2003,
	// Error 2013: Lost connection to the master during a query (TCP timeout or network blip).
	2013,
	// Error 1158: Got an error reading communication packets
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1100-to-1199/e1158
	1158,
	// Error 2026: TLS/SSL handshake failed (usually expired certificates)
	2026,
	// Error 1045: Access denied for user (using password)
	// https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1045
	1045,
	// Error 1130: Host is not allowed to connect to this MariaDB server
	// https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1100-to-1199/e1130
	1130,
	// Error 1129: Host is blocked because of many connection errors
	// https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1100-to-1199/e1129
	1129,
	// Error 1040: Too many connections
	// https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1040
	1040,
}

func shouldReconcileReplicaRecovery(mdb *mariadbv1alpha1.MariaDB) bool {
	if !mdb.IsReplicationEnabled() || !mdb.HasConfiguredReplication() {
		return false
	}
	// Replica clusters part of a multi-cluster topology must be recovered using PhysicalBackups taken in in the primary cluster.
	if mdb.IsMultiClusterReplica() {
		return false
	}
	if mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() || mdb.IsInitializing() || mdb.IsScalingOut() ||
		mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsUpdating() {
		return false
	}
	return true
}

func (r *MariaDBReconciler) reconcileReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {

	logger := log.FromContext(ctx).
		WithName("replica-recovery")
	logger.Info("ReconcileReplicaRecovery")

	if !shouldReconcileReplicaRecovery(mariadb) {
		logger.Info("Should not reconcile replica recovery")
		return ctrl.Result{}, nil
	}
	if !mariadb.IsReplicaRecoveryEnabled() {
		logger.Info("Replica recovery is not enabled")
		if err := r.resetReplicaRecovery(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !mariadb.IsRecoveringReplicas() || mariadb.ReplicaRecoveryError() != nil {
		isRecovering := mariadb.IsRecoveringReplicas()
		recoveryError := mariadb.ReplicaRecoveryError()
		logger.Info("Is not recovering replicas or there is an error, so we can't reconcile replica recovery",
			"isRecovering", isRecovering, "recoveryError", recoveryError)
		if result, err := r.reconcileReplicaRecoveryError(ctx, mariadb, logger); !result.IsZero() || err != nil {
			logger.Info("reconcile replica recovery error failed", "result", result, "err", err)
			return result, err
		}
	}

	replication := mariadb.Replication()

	if replication.IsExternalReplication() {
		//Check for skippable SQL Errors (user and privileges relates), and skip the error
		replicasToSkipReplicationError := getReplicasToSkipReplicationError(mariadb, logger)
		if _, err := r.reconcileSkippableReplicationError(ctx, replicasToSkipReplicationError, mariadb, logger); err != nil {
			logger.Info("ExternalReplication, logical restore error", "error", err)
			return ctrl.Result{}, err
		}
	}

	replicasToRecover := getReplicasToRecover(mariadb, logger)
	logger = logger.
		WithValues("replicas", replicasToRecover)

	logger.Info("Replicas to recover", "total replicas", replicasToRecover)
	if len(replicasToRecover) == 0 {
		return ctrl.Result{}, nil
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplicaRecovering(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.V(1).Info("Recovering replicas")
	physicalBackupKey := mariadb.PhysicalBackupReplicaRecoveryKey()

	if result, err := r.reconcileReplicaPhysicalBackup(ctx, physicalBackupKey, mariadb, logger); !result.IsZero() || err != nil {
		if replication.IsExternalReplication() {

			if err != nil && errors.Is(err, errPhysicalBackupJobLaunchTimeout) {
				logger.Info("ExternalReplication, PhysicalBackup not able to launch jobs, trigger LogicalBackup", "error", err)
				if result, err := r.reconcileLogicalBackup(ctx, mariadb, replication, logger); err != nil || !result.IsZero() {
					return result, err
				}
				if _, err := r.reconcileLogicalBackupReplicaRecovery(ctx, replicasToRecover[0], mariadb, logger); err != nil {
					logger.Info("ExternalReplication, logical restore error", "error", err)
					return ctrl.Result{}, err
				}

				// Remove current physical backup as logical backup was required to avoid reaching the timeout again
				_ = r.cleanupPhysicalBackup(ctx, mariadb.PhysicalBackupReplicaRecoveryKey())
				logger.Info("ExternalReplication, logical restore finished - requeue in 1min")
				return ctrl.Result{}, nil
			}
			logger.Info("ExternalReplication, logical restore not required")
			return ctrl.Result{}, nil
		}
		return result, err
	}
	logger.Info("PhysicalBackup finished")
	physicalBackup, err := r.getPhysicalBackup(ctx, physicalBackupKey, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	snapshotKey, err := r.getVolumeSnapshotKey(ctx, mariadb, physicalBackup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot key: %v", err)
	}

	return r.reconcileReplicasToRecover(ctx, replicasToRecover, mariadb, physicalBackup, snapshotKey, logger)
}

func (r *MariaDBReconciler) reconcileReplicaRecoveryError(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})

	if replication.Replica.ReplicaBootstrapFrom == nil {
		r.Recorder.Eventf(mariadb,
			nil,
			corev1.EventTypeWarning,
			mariadbv1alpha1.ReasonMariaDBReplicaRecoveryError,
			mariadbv1alpha1.ActionReconciling,
			"Unable to recover replicas: replica datasource not found (replication.replica.bootstrapFrom is nil)",
		)

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetReplicaRecoveryError(status, "replica datasource not found (replication.replica.bootstrapFrom is nil)")
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}

		logger.Info("Unable to recover replicas: replica datasource not found (replication.replica.bootstrapFrom is nil). Requeuing...")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileReplicasToRecover(ctx context.Context, replicas []string, mariadb *mariadbv1alpha1.MariaDB,
	physicalBackup *mariadbv1alpha1.PhysicalBackup, snapshotKey *types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Reconcile replicas to recover")
	for _, replica := range replicas {
		replicaLogger := logger.WithValues("replica", replica)
		replicaLogger.V(1).Info("Recovering replica")

		if snapshotKey != nil {
			if result, err := r.reconcileSnapshotReplicaRecovery(
				ctx,
				replica,
				physicalBackup,
				mariadb,
				snapshotKey,
				replicaLogger.WithName("snapshot"),
			); !result.IsZero() || err != nil {
				return result, err
			}
		} else {
			if result, err := r.reconcileJobReplicaRecovery(
				ctx,
				replica,
				physicalBackup,
				mariadb,
				replicaLogger.WithName("job"),
			); !result.IsZero() || err != nil {
				return result, err
			}
		}

		if err := r.ensureReplicationConfiguredInPod(ctx, replica, mariadb, snapshotKey, replicaLogger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring replica %s configured: %v", replica, err)
		}
		if err := r.ensureReplicaRecovered(ctx, replica, mariadb, replicaLogger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring replica %s recovered: %v", replica, err)
		}
	}
	logger.Info("Replicas to recovered, cleaning up")
	if err := r.setReplicaRecoveredAndCleanup(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to track replication status
	return ctrl.Result{Requeue: true}, nil
}

func (r *MariaDBReconciler) reconcileJobReplicaRecovery(ctx context.Context, replica string, physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {
	podKey := types.NamespacedName{
		Name:      replica,
		Namespace: mariadb.Namespace,
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		mariadb.SetReplicaToRecover(&replica)
		mariadb.Status.Replication.Roles[replica] = mariadbv1alpha1.ReplicationRoleUnknown
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if _, err := r.reconcileService(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}

	isPodInitializing, err := r.isPodInitializing(ctx, podKey)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking Pod initializing: %v", err)
	}
	if !isPodInitializing {
		logger.Info("Restarting Pod")
		if err := r.ensurePodInitializing(ctx, podKey, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring Pod initializing: %v", err)
		}
	}

	if result, err := r.reconcileAndWaitForRecoveryJob(
		ctx,
		physicalBackup,
		mariadb,
		podKey,
		logger,
	); !result.IsZero() || err != nil {
		return result, err
	}
	// Wait for pod get ready

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		mariadb.SetReplicaToRecover(nil)
		mariadb.Status.Replication.Roles[replica] = mariadbv1alpha1.ReplicationRoleReplica
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileLogicalBackupReplicaRecovery(ctx context.Context, replica string,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {

	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
	}

	if _, err := r.reconcileRestoreInPod(ctx, mariadb, *podIndex, logger, true); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling restore in Pod: %v", err)
	}

	logger.Info("cleaning up the restore pod")
	_ = r.cleanupRestoreInPod(ctx, mariadb, *podIndex, logger)

	logger.Info("reconciling external logical restore finished")

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileSnapshotReplicaRecovery(ctx context.Context, replica string,
	physicalBackup *mariadbv1alpha1.PhysicalBackup, mariadb *mariadbv1alpha1.MariaDB, snapshotKey *types.NamespacedName,
	logger logr.Logger) (ctrl.Result, error) {
	if snapshotKey == nil {
		return ctrl.Result{}, errors.New("VolumeSnapshot key must be set")
	}
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
	}
	podKey := types.NamespacedName{
		Name:      replica,
		Namespace: mariadb.Namespace,
	}
	pvcKey := mariadb.PVCKey(builder.StorageVolume, *podIndex)

	if result, err := r.waitForReadyVolumeSnapshot(ctx, *snapshotKey, logger); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.deleteStatefulSetLeavingOrphanPods(ctx, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting StatefulSet: %v", err)
	}
	defer func() {
		// requeuing not handled, as it only applies to updates
		if _, err := r.reconcileStatefulSet(ctx, mariadb); err != nil {
			logger.Error(err, "error reconciling StatefulSet: %v", err)
		}
	}()

	deletePodCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	if err := wait.PollUntilSuccessOrContextCancelWithInterval(deletePodCtx, 5*time.Second, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, podKey, &pod); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Pod deleted")
				return nil
			}
			return err
		}
		if err := r.Delete(ctx, &pod); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Pod deleted")
				return nil
			}
			return err
		}
		return errors.New("Pod still exists") //nolint:staticcheck
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting Pod: %v", err)
	}

	deletePVCCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	if err := wait.PollUntilSuccessOrContextCancelWithInterval(deletePVCCtx, 5*time.Second, logger, func(ctx context.Context) error {
		var pvc corev1.PersistentVolumeClaim
		if err := r.Get(ctx, pvcKey, &pvc); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("PVC deleted")
				return nil
			}
			return err
		}
		if err := r.Delete(ctx, &pvc); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("PVC deleted")
				return nil
			}
			return err
		}
		return errors.New("PVC still exists") //nolint:staticcheck
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting PVC: %v", err)
	}

	logger.Info("Provisioning new PVC from VolumeSnapshot", "snapshot", snapshotKey.Name)
	if err := r.reconcilePVC(ctx, mariadb, pvcKey, builder.WithVolumeSnapshotDataSource(snapshotKey.Name)); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling PVC: %v", err)
	}

	logger.Info("Provisioning new Pod")
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) isPodInitializing(ctx context.Context, key types.NamespacedName) (bool, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return false, fmt.Errorf("error getting Pod %s: %v", key.Name, err)
	}
	return podobj.PodInitializing(&pod), nil
}

func (r *MariaDBReconciler) ensurePodInitializing(ctx context.Context, key types.NamespacedName, logger logr.Logger) error {
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return fmt.Errorf("error getting Pod %s: %v", key.Name, err)
	}
	if podobj.PodInitializing(&pod) {
		return nil
	}

	oldUID := pod.UID
	if err := r.Delete(ctx, &pod); err != nil {
		return fmt.Errorf("error deleting Pod: %v", err)
	}

	pollCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancelWithInterval(pollCtx, 5*time.Second, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, key, &pod); err != nil {
			return fmt.Errorf("error getting Pod %s: %v", key.Name, err)
		}
		if pod.UID == oldUID {
			return errors.New("old Pod still present")
		}
		if podobj.PodInitializing(&pod) {
			return nil
		}
		return errors.New("Pod not initializing") //nolint:staticcheck
	})
}

func (r *MariaDBReconciler) reconcileAndWaitForRecoveryJob(ctx context.Context, physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, podKey types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	podIndex, err := stsobj.PodIndex(podKey.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting Pod index: %v", err)
	}

	var pod corev1.Pod
	if err := r.Get(ctx, podKey, &pod); err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting Pod: %v", err)
	}
	if pod.Spec.NodeName == "" {
		return ctrl.Result{}, errors.New("Pod must be scheduled: spec.nodeName is empty") //nolint:staticcheck
	}

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	bootstrapFrom := ptr.Deref(replication.Replica.ReplicaBootstrapFrom, mariadbv1alpha1.ReplicaBootstrapFrom{})

	if replication.IsExternalReplication() {
		emdb, err := r.RefResolver.ExternalMariaDB(ctx, &replication.ReplicaFromExternal.MariaDBRef.ObjectReference, mariadb.Namespace)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting external MariaDB: %v", err)
		}

		var binlogExpireLogsDuration time.Duration

		logger.Info("Getting the binlog_expire_logs_seconds on the external MariaDB")
		if binlogExpireLogsDuration, err = getBinlogExpireLogsDuration(emdb, ctx, r.RefResolver, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
		}

		ageThreshold := time.Now().Add(-binlogExpireLogsDuration)

		return r.reconcileAndWaitForInitJob(
			ctx,
			mariadb,
			mariadb.PhysicalBackupInitJobKey(*podIndex),
			*podIndex,
			logger,
			builder.WithPhysicalBackup(
				physicalBackup,
				time.Now(),
				bootstrapFrom.RestoreJob,
				command.WithCleanupDataDir(true),
			),
			builder.WithReplicaRecovery(&pod),
			builder.WithAgeThreshold(&ageThreshold),
		)
	}

	return r.reconcileAndWaitForInitJob(
		ctx,
		mariadb,
		mariadb.PhysicalBackupInitJobKey(*podIndex),
		*podIndex,
		logger,
		builder.WithPhysicalBackup(
			physicalBackup,
			time.Now(),
			bootstrapFrom.RestoreJob,
			command.WithCleanupDataDir(true),
		),
		builder.WithReplicaRecovery(&pod),
	)

}

func (r *MariaDBReconciler) ensureReplicaRecovered(ctx context.Context, replica string, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) error {
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return fmt.Errorf("error getting replica pod index: %v", err)
	}

	pollCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancel(pollCtx, logger, func(ctx context.Context) error {
		client, err := sql.NewInternalClientWithPodIndex(ctx, mariadb, r.RefResolver, *podIndex)
		if err != nil {
			return fmt.Errorf("error getting SQL client: %v", err)
		}
		defer client.Close()

		replStatus, err := client.ReplicaStatus(ctx, logger)
		if err != nil {
			return fmt.Errorf("error getting replica status: %v", err)
		}

		//ensure pod is on ready state
		pod := corev1.Pod{}
		podKey := types.NamespacedName{
			Name:      replica,
			Namespace: mariadb.Namespace,
		}

		if err := r.Get(ctx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting replica pod: %v", err)
		}

		if !podobj.PodReady(&pod) {
			return errors.New("pod not ready")
		}

		if replStatus.LastIOErrno != nil && *replStatus.LastIOErrno == 0 &&
			replStatus.LastSQLErrno != nil && *replStatus.LastSQLErrno == 0 {
			logger.Info("Replica recovered")
			return nil
		}
		return errors.New("replica not recovered")
	})
}

func (r *MariaDBReconciler) setReplicaRecoveredAndCleanup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !mariadb.IsRecoveringReplicas() {
		return nil
	}
	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplicaRecovered(status)
		mariadb.SetReplicaToRecover(nil)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if err := r.cleanupInitJobs(ctx, mariadb); err != nil {
		return err
	}
	return nil
}

func (r *MariaDBReconciler) resetReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		mariadb.Status.RemoveCondition(mariadbv1alpha1.ConditionTypeReplicaRecovered)
		mariadb.SetReplicaToRecover(nil)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

func getReplicasToRecover(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) []string {
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	var replicas []string
	for replica, err := range replication.Replicas {
		logger.Info("Check if it is a recoverable error", "replica", replica)
		if isRecoverableError(
			mdb,
			err,
			recoverableIOErrorCodes,
			recoverableSQLErrorCodes,
			logger.WithValues("replica", replica),
		) {
			replicas = append(replicas, replica)
		}
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] < replicas[j]
	})
	return replicas
}

func getReplicasToSkipReplicationError(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) []string {
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	var replicas []string
	for replica, err := range replication.Replicas {
		logger.Info("Check if it is a skippable error", "replica", replica)
		if isSkippableError(
			mdb,
			err,
			externalUserPrivilegesSkippableSQLErrorCodes,
			logger.WithValues("replica", replica),
		) {
			replicas = append(replicas, replica)
		}
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] < replicas[j]
	})
	return replicas
}

func isRecoverableError(mdb *mariadbv1alpha1.MariaDB, status mariadbv1alpha1.ReplicaStatus,
	recoverableIOErrorCodes []int, recoverableSQLErrorCodes []int, logger logr.Logger) bool {
	for _, code := range recoverableIOErrorCodes {
		if status.LastIOErrno != nil && *status.LastIOErrno == code {
			logger.V(1).Info("Recoverable IO error code detected", "io-errno", *status.LastIOErrno)
			logger.Info("Recoverable IO error code detected", "io-errno", *status.LastIOErrno)
			return true
		}
	}
	for _, code := range recoverableSQLErrorCodes {
		if status.LastSQLErrno != nil && *status.LastSQLErrno == code {
			logger.V(1).Info("Recoverable SQL error code detected", "sql-errno", *status.LastSQLErrno)
			logger.Info("Recoverable SQL error code detected", "sql-errno", *status.LastSQLErrno)
			return true
		}
	}

	for _, code := range notRecoverableIOErrorCodes {
		if status.LastIOErrno != nil && *status.LastIOErrno == code {
			logger.V(1).Info("Not recoverable IO error code detected", "io-errno", *status.LastIOErrno)
			logger.Info("Not recoverable IO error code detected", "io-errno", *status.LastIOErrno)
			return false
		}
	}

	lastIOErrno := ptr.Deref(status.LastIOErrno, 0)
	lastSQLErrno := ptr.Deref(status.LastSQLErrno, 0)

	if (lastIOErrno != 0 || lastSQLErrno != 0) && !status.LastErrorTransitionTime.IsZero() {
		logger.Info("Non recoverable error", "lastIOErrno", lastIOErrno,
			"lastSQLErrno", lastSQLErrno, "LastErrorTransitionTime", status.LastErrorTransitionTime)
		replication := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
		recovery := ptr.Deref(replication.Replica.ReplicaRecovery, mariadbv1alpha1.ReplicaRecovery{})
		errThreshold := ptr.Deref(recovery.ErrorDurationThreshold, metav1.Duration{Duration: 5 * time.Minute})
		age := time.Since(status.LastErrorTransitionTime.Time)

		logger.Info(
			"Current error",
			"io-errno", lastIOErrno,
			"sql-errno", lastSQLErrno,
			"age", age,
			"threshold", errThreshold.Duration,
		)
		if age > errThreshold.Duration {
			logger.Info(
				"Error surpassed threshold",
				"io-errno", lastIOErrno,
				"sql-errno", lastSQLErrno,
				"age", age,
				"threshold", errThreshold.Duration,
			)
			return true
		}
	}
	return false
}

func isSkippableError(mdb *mariadbv1alpha1.MariaDB, status mariadbv1alpha1.ReplicaStatus,
	externalUserPrivilegesSkippableSQLErrorCodes []int, logger logr.Logger) bool {
	for _, code := range externalUserPrivilegesSkippableSQLErrorCodes {
		if status.LastSQLErrno != nil && *status.LastSQLErrno == code {
			logger.V(1).Info("Skippable SQL error code detected", "sql-errno", *status.LastSQLErrno)
			return true
		}
	}
	return false
}

func (r *MariaDBReconciler) reconcileSkippableReplicationError(ctx context.Context, replicasToSkip []string,
	mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {

	for _, replica := range replicasToSkip {
		logger.V(1).Info("Skip SQL Replica Error on pod", "pod", replica)
		podIndex, err := stsobj.PodIndex(replica)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
		}
		client, err := sql.NewInternalClientWithPodIndex(ctx, mdb, r.RefResolver, *podIndex)
		if err != nil {
			logger.V(1).Info("error getting replica client", "err", err, "pod", *podIndex)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		defer client.Close()

		if err := client.StopSlave(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("error stopping slave: %v", err)
		}

		if err := client.SetSqlSlaveSkipCounter(ctx, 1); err != nil {
			return ctrl.Result{}, fmt.Errorf("error setting skip counter: %v", err)
		}

		if err := client.StartSlave(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("error starting slave: %v", err)
		}
	}
	return ctrl.Result{}, nil
}
