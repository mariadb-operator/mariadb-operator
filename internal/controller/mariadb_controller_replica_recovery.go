package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/command"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	podobj "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/wait"
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
}

func shouldReconcileReplicaRecovery(mdb *mariadbv1alpha1.MariaDB) bool {
	if !mdb.IsReplicationEnabled() || !mdb.HasConfiguredReplication() {
		return false
	}
	if mdb.IsSwitchingPrimary() || mdb.IsInitializing() || mdb.IsScalingOut() ||
		mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsUpdating() {
		return false
	}
	return true
}

func (r *MariaDBReconciler) reconcileReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcileReplicaRecovery(mariadb) {
		return ctrl.Result{}, nil
	}
	if !mariadb.IsReplicaRecoveryEnabled() {
		if err := r.resetReplicaRecovery(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).
		WithName("replica-recovery")
	replicasToRecover := getReplicasToRecover(mariadb, logger)
	logger = logger.
		WithValues("replicas", replicasToRecover)

	if len(replicasToRecover) == 0 {
		if err := r.setReplicaRecoveredAndCleanup(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
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
		return result, err
	}
	physicalBackup, err := r.getPhysicalBackup(ctx, physicalBackupKey, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	snapshotKey, err := r.getVolumeSnapshotKey(ctx, mariadb, physicalBackup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot key: %v", err)
	}

	for _, replica := range replicasToRecover {
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
	// Requeue to track replication status
	return ctrl.Result{Requeue: true}, nil
}

func (r *MariaDBReconciler) reconcileJobReplicaRecovery(ctx context.Context, replica string, physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
	}
	podKey := types.NamespacedName{
		Name:      replica,
		Namespace: mariadb.Namespace,
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		mariadb.SetReplicaToRecover(&replica)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
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

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	bootstrapFrom := ptr.Deref(replication.Replica.ReplicaBootstrapFrom, mariadbv1alpha1.ReplicaBootstrapFrom{})

	if result, err := r.reconcileAndWaitForInitJob(
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
	); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		mariadb.SetReplicaToRecover(nil)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
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
	if err := r.Delete(ctx, &pod); err != nil {
		return fmt.Errorf("error deleting Pod: %v", err)
	}

	pollCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancelWithInterval(pollCtx, 30*time.Second, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, key, &pod); err != nil {
			return fmt.Errorf("error getting Pod %s: %v", key.Name, err)
		}
		if podobj.PodInitializing(&pod) {
			return nil
		}

		if err := r.Delete(ctx, &pod); err != nil {
			return err
		}
		return errors.New("Pod not initializing") //nolint:staticcheck
	})
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

		replErrors, err := client.ReplicaErrors(ctx)
		if err != nil {
			return fmt.Errorf("error getting replica errors: %v", err)
		}

		if replErrors.LastIOErrno != nil && *replErrors.LastIOErrno == 0 &&
			replErrors.LastSQLErrno != nil && *replErrors.LastSQLErrno == 0 {
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

	if err := r.cleanupPhysicalBackup(ctx, mariadb.PhysicalBackupReplicaRecoveryKey()); err != nil {
		return err
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
	for replica, err := range replication.Errors {
		if isRecoverableError(mdb, err, logger.WithValues("replica", replica)) {
			replicas = append(replicas, replica)
		}
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] < replicas[j]
	})
	return replicas
}

func isRecoverableError(mdb *mariadbv1alpha1.MariaDB, status mariadbv1alpha1.ReplicaErrorStatus,
	logger logr.Logger) bool {
	for _, code := range recoverableIOErrorCodes {
		if status.LastIOErrno != nil && *status.LastIOErrno == code {
			logger.V(1).Info("Recoverable IO error code detected", "io-errno", *status.LastIOErrno)
			return true
		}
	}
	lastIOErrno := ptr.Deref(status.LastIOErrno, 0)
	lastSQLErrno := ptr.Deref(status.LastSQLErrno, 0)

	if (lastIOErrno != 0 || lastSQLErrno != 0) && !status.LastTransitionTime.IsZero() {
		replication := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
		recovery := ptr.Deref(replication.Replica.ReplicaRecovery, mariadbv1alpha1.ReplicaRecovery{})
		errThreshold := ptr.Deref(recovery.ErrorDurationThreshold, metav1.Duration{Duration: 5 * time.Minute})
		age := time.Since(status.LastTransitionTime.Time)

		logger.V(1).Info(
			"Current error",
			"io-errno", lastIOErrno,
			"sql-errno", lastSQLErrno,
			"age", age,
			"threshold", errThreshold.Duration,
		)
		if age > errThreshold.Duration {
			logger.V(1).Info(
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
