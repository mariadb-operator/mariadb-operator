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
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	podobj "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	stsobj "github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/wait"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var nonRecoverableIOErrorCodes = []int{
	// Error 1236: Got fatal error from master when reading data from binary log.
	// See: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1200-to-1299/e1236
	1236,
}

func (r *MariaDBReconciler) reconcileReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsReplicaRecoveryEnabled() {
		return ctrl.Result{}, nil
	}
	replicasToRecover := getReplicasToRecover(mariadb)
	logger := log.FromContext(ctx).
		WithName("replica-recovery").
		WithValues("replicas", replicasToRecover)

	if len(replicasToRecover) == 0 {
		if mariadb.IsRecoveringReplicas() {
			logger.Info("All replicas have been recovered")

			if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
				condition.SetReplicaRecovered(status)
				return nil
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
			}

			// TODO: cleanup PhysicalBackup and init Jobs
		}
		return ctrl.Result{}, nil
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplicaRecovering(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.Info("Recovering replicas")

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
			if result, err := r.reconcileJobReplicaRecovery(ctx, mariadb, replica, logger); !result.IsZero() || err != nil {
				return result, err
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileJobReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, replica string,
	logger logr.Logger) (ctrl.Result, error) {
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
	}
	pvcKey := mariadb.PVCKey(builder.StorageVolume, *podIndex)
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

	// TODO: update initcontainer to check replica to recover
	isPodInitializing, err := r.isPodInitializing(ctx, podKey)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking Pod initializing: %v", err)
	}
	if !isPodInitializing {
		if err := r.ensurePVCTerminating(ctx, pvcKey, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring PVC terminating: %v", err)
		}
		if err := r.ensurePodInitializing(ctx, podKey, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring Pod initializing: %v", err)
		}
	}

	// if result, err := r.reconcileAndWaitForInitJob(
	// 	ctx,
	// 	mariadb,
	// 	physicalBackupKey,
	// 	podIndex,
	// 	opts.restoreOpts...,
	// ); !result.IsZero() || err != nil {
	// 	return result, err
	// }

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		mariadb.SetReplicaToRecover(nil)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	// TODO: prevent replication reconciliation from configuring replica to recover
	// SetReplicasToConfigure

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) isPodInitializing(ctx context.Context, key types.NamespacedName) (bool, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return false, fmt.Errorf("error getting Pod %s: %v", key.Name, err)
	}
	return podobj.PodInitializing(&pod), nil
}

func (r *MariaDBReconciler) ensurePVCTerminating(ctx context.Context, key types.NamespacedName, logger logr.Logger) error {
	isTerminated, err := r.isPVCTerminated(ctx, key, logger)
	if isTerminated {
		return nil
	}
	if err != nil {
		return fmt.Errorf("error checking PVC %s terminated: %v", key.Name, err)
	}

	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, key, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting PVC %s: %v", key.Name, err)
	}
	if err := r.Delete(ctx, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete PVC %s: %w", key.Name, err)
	}

	pollCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancel(pollCtx, logger, func(ctx context.Context) error {
		isTerminated, err := r.isPVCTerminated(ctx, key, logger)
		if isTerminated {
			return nil
		}
		if err != nil {
			return fmt.Errorf("error checking PVC %s terminated: %v", key.Name, err)
		}
		return errors.New("PVC not terminated")
	})
}

func (r *MariaDBReconciler) isPVCTerminated(ctx context.Context, key types.NamespacedName, logger logr.Logger) (bool, error) {
	var pvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, key, &pvc)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get PVC %s: %w", key.Name, err)
	}
	if pvc.DeletionTimestamp != nil {
		return true, nil
	}
	return false, nil
}

func (r *MariaDBReconciler) ensurePodInitializing(ctx context.Context, key types.NamespacedName, logger logr.Logger) error {
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return fmt.Errorf("error getting Pod %s: %v", key.Name, err)
	}
	if err := r.Delete(ctx, &pod); err != nil {
		return fmt.Errorf("error deleting Pod %s: %v", key.Name, err)
	}

	pollCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancel(pollCtx, logger, func(ctx context.Context) error {
		if err := r.Get(ctx, key, &pod); err != nil {
			return fmt.Errorf("error getting Pod %s: %v", key.Name, err)
		}
		if podobj.PodInitializing(&pod) {
			return nil
		}
		if err := r.Delete(ctx, &pod); err != nil {
			return err
		}
		return errors.New("Pod not initializing")
	})
}

func getReplicasToRecover(mdb *mariadbv1alpha1.MariaDB) []string {
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	var replicas []string
	for replica, err := range replication.Errors {
		if isNonRecoverableError(err) {
			replicas = append(replicas, replica)
		}
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] > replicas[j]
	})
	return replicas
}

func isNonRecoverableError(s mariadbv1alpha1.ReplicaErrorStatus) bool {
	if s.LastIOErrno == nil {
		return false
	}
	for _, code := range nonRecoverableIOErrorCodes {
		if *s.LastIOErrno == code {
			return true
		}
	}
	return false
}
