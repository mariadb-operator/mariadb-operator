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

func (r *MariaDBReconciler) reconcileReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsReplicationEnabled() || !mariadb.HasConfiguredReplication() {
		return ctrl.Result{}, nil
	}
	if !mariadb.IsReplicaRecoveryEnabled() {
		if err := r.resetReplicaRecovery(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	replicasToRecover := getReplicasToRecover(mariadb)
	logger := log.FromContext(ctx).
		WithName("replica-recovery").
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
		logger.Info("Recovering replica", "replica", replica)
		if snapshotKey == nil {
			if result, err := r.reconcileJobReplicaRecovery(ctx, replica, physicalBackup, mariadb, logger); !result.IsZero() || err != nil {
				return result, err
			}
		}
		if err := r.ensureReplicaConfigured(ctx, replica, mariadb, snapshotKey, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring replica %s configured: %v", replica, err)
		}
		if err := r.ensureReplicaRecovered(ctx, replica, mariadb, logger); err != nil {
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

	pollCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
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

func getReplicasToRecover(mdb *mariadbv1alpha1.MariaDB) []string {
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	var replicas []string
	for replica, err := range replication.Errors {
		if isRecoverableError(err) {
			replicas = append(replicas, replica)
		}
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] < replicas[j]
	})
	return replicas
}

func isRecoverableError(s mariadbv1alpha1.ReplicaErrorStatus) bool {
	if s.LastIOErrno == nil {
		return false
	}
	for _, code := range recoverableIOErrorCodes {
		if *s.LastIOErrno == code {
			return true
		}
	}
	return false
}
