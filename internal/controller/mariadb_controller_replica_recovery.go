package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/command"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/wait"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	if mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() || mdb.IsInitializing() || mdb.IsScalingOut() ||
		mdb.IsRestoringBackup() || mdb.IsResizingStorage() {
		return false
	}
	return true
}

func (r *MariaDBReconciler) reconcileReplicaRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcileReplicaRecovery(mariadb) {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).
		WithName("replica-recovery")

	pvcStates, pvcUIDs, pvcRecoveryReplicas, err := r.getPVCRecoveryReplicas(ctx, mariadb, logger)
	if err != nil {
		return ctrl.Result{}, err
	}
	replicaRecoveryEnabled := mariadb.IsReplicaRecoveryEnabled()
	immediateRecoveryReplicas := getReplicasWithImmediateRecoverableErrors(mariadb, logger)

	if handled, err := r.resetReplicaRecoveryIfNotNeeded(
		ctx,
		mariadb,
		replicaRecoveryEnabled || len(immediateRecoveryReplicas) > 0,
		pvcRecoveryReplicas,
		pvcUIDs,
	); handled || err != nil {
		return ctrl.Result{}, err
	}

	if !mariadb.IsRecoveringReplicas() || mariadb.ReplicaRecoveryError() != nil {
		if result, err := r.reconcileReplicaRecoveryError(ctx, mariadb, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	replicasToRecover := mergeReplicasToRecover(
		immediateRecoveryReplicas,
		pvcRecoveryReplicas,
	)
	if replicaRecoveryEnabled {
		replicasToRecover = mergeReplicasToRecover(
			getReplicasToRecover(mariadb, logger),
			replicasToRecover,
		)
	}
	logger = logger.
		WithValues("replicas", replicasToRecover)

	if handled, err := r.completeReplicaRecoveryIfDone(ctx, mariadb, replicasToRecover, pvcUIDs); handled || err != nil {
		return ctrl.Result{}, err
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplicaRecovering(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.V(1).Info("Recovering replicas")
	if result, err := r.quiescePVCRecoveryReplicas(ctx, mariadb, pvcRecoveryReplicas, logger); !result.IsZero() || err != nil {
		return result, err
	}
	physicalBackupKey := mariadb.PhysicalBackupReplicaRecoveryKey()
	physicalBackup, snapshotKey, result, err := r.reconcileReplicaRecoveryBackup(
		ctx,
		physicalBackupKey,
		mariadb,
		pvcStates,
		pvcRecoveryReplicas,
		logger,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !result.IsZero() {
		return result, nil
	}

	return r.reconcileReplicasToRecover(ctx, replicasToRecover, mariadb, physicalBackup, snapshotKey, logger)
}

func (r *MariaDBReconciler) getPVCRecoveryReplicas(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (map[int]storagePVCState, map[int]string, []string, error) {
	pvcStates, err := r.getStoragePVCStates(ctx, mariadb)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting storage PVC state: %v", err)
	}

	pvcUIDs := make(map[int]string, len(pvcStates))
	for i, state := range pvcStates {
		if state.UID != "" {
			pvcUIDs[i] = state.UID
		}
	}
	replicas := mergeReplicasToRecover(
		getReplicasWithLostPVC(mariadb, pvcUIDs, logger),
		getReplicasWithFreshPVCReplicationErrors(mariadb, pvcStates, logger),
	)
	return pvcStates, pvcUIDs, replicas, nil
}

func (r *MariaDBReconciler) resetReplicaRecoveryIfNotNeeded(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replicaRecoveryEnabled bool, pvcRecoveryReplicas []string, pvcUIDs map[int]string) (bool, error) {
	if replicaRecoveryEnabled || len(pvcRecoveryReplicas) > 0 {
		return false, nil
	}
	if err := r.resetReplicaRecovery(ctx, mariadb); err != nil {
		return false, err
	}
	if err := r.syncStoragePVCUIDAnnotations(ctx, mariadb, pvcUIDs); err != nil {
		return false, fmt.Errorf("error syncing storage PVC annotations: %v", err)
	}
	if err := r.clearReplicaRecoveryRefreshPVCUIDAnnotations(ctx, mariadb); err != nil {
		return false, fmt.Errorf("error clearing replica recovery retry annotations: %v", err)
	}
	if err := r.clearReplicaRecoveryNodeAnnotations(ctx, mariadb); err != nil {
		return false, fmt.Errorf("error clearing replica recovery node annotations: %v", err)
	}
	if err := r.clearReplicaRecoveryCompletedPVCUIDAnnotations(ctx, mariadb); err != nil {
		return false, fmt.Errorf("error clearing replica recovery completed PVC annotations: %v", err)
	}
	if err := r.cleanupReplicaRecoveryArtifacts(ctx, mariadb); err != nil {
		return false, fmt.Errorf("error cleaning replica recovery artifacts: %v", err)
	}
	return true, nil
}

func (r *MariaDBReconciler) completeReplicaRecoveryIfDone(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replicasToRecover []string, pvcUIDs map[int]string) (bool, error) {
	if len(replicasToRecover) > 0 {
		return false, nil
	}
	if err := r.setReplicaRecoveredAndCleanup(ctx, mariadb); err != nil {
		return false, err
	}
	if err := r.syncStoragePVCUIDAnnotations(ctx, mariadb, pvcUIDs); err != nil {
		return false, fmt.Errorf("error syncing storage PVC annotations: %v", err)
	}
	return true, nil
}

func (r *MariaDBReconciler) reconcileReplicaRecoveryBackup(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState, pvcRecoveryReplicas []string,
	logger logr.Logger) (*mariadbv1alpha1.PhysicalBackup, *types.NamespacedName, ctrl.Result, error) {
	if result, err := r.ensureReplicaPhysicalBackupCurrent(
		ctx,
		key,
		mariadb,
		pvcStates,
		pvcRecoveryReplicas,
		logger,
	); !result.IsZero() || err != nil {
		return nil, nil, result, err
	}
	if result, err := r.reconcileReplicaPhysicalBackup(ctx, key, mariadb, logger); !result.IsZero() || err != nil {
		return nil, nil, result, err
	}

	physicalBackup, err := r.getPhysicalBackup(ctx, key, mariadb)
	if err != nil {
		return nil, nil, ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	snapshotKey, err := r.getVolumeSnapshotKey(ctx, mariadb, physicalBackup)
	if err != nil {
		return nil, nil, ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot key: %v", err)
	}
	return physicalBackup, snapshotKey, ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcileReplicaRecoveryError(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	if recoveryErr := mariadb.ReplicaRecoveryError(); recoveryErr != nil &&
		!strings.Contains(recoveryErr.Error(), "replica datasource not found") {
		logger.Info("Unable to recover replicas. Requeuing...", "err", recoveryErr.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})

	if replication.Replica.ReplicaBootstrapFrom == nil {
		if r.Recorder != nil {
			r.Recorder.Eventf(mariadb,
				nil,
				corev1.EventTypeWarning,
				mariadbv1alpha1.ReasonMariaDBReplicaRecoveryError,
				mariadbv1alpha1.ActionReconciling,
				"Unable to recover replicas: replica datasource not found (replication.replica.bootstrapFrom is nil)",
			)
		}

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

func (r *MariaDBReconciler) ensureReplicaPhysicalBackupCurrent(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState, pvcRecoveryReplicas []string,
	logger logr.Logger) (ctrl.Result, error) {
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	if !isReplicaPhysicalBackupStale(&physicalBackup, mariadb, pvcStates, pvcRecoveryReplicas) {
		return ctrl.Result{}, nil
	}

	logger.Info("Deleting stale replica recovery artifacts", "physicalBackup", physicalBackup.Name)
	if err := r.cleanupReplicaRecoveryArtifacts(ctx, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting stale replica recovery artifacts: %v", err)
	}
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func isReplicaPhysicalBackupStale(physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState, pvcRecoveryReplicas []string) bool {
	if isReplicaPhysicalBackupStaleByCondition(physicalBackup, mariadb) {
		return true
	}
	return isReplicaPhysicalBackupStaleForPVCRecovery(physicalBackup, pvcStates, pvcRecoveryReplicas)
}

func isReplicaPhysicalBackupStaleByCondition(physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) bool {
	condition := meta.FindStatusCondition(mariadb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		return false
	}
	if condition.LastTransitionTime.IsZero() || physicalBackup.CreationTimestamp.IsZero() {
		return false
	}
	return physicalBackup.CreationTimestamp.Time.Before(condition.LastTransitionTime.Time)
}

func isReplicaPhysicalBackupStaleForPVCRecovery(physicalBackup *mariadbv1alpha1.PhysicalBackup,
	pvcStates map[int]storagePVCState, pvcRecoveryReplicas []string) bool {
	if physicalBackup.CreationTimestamp.IsZero() {
		return false
	}

	for _, replica := range pvcRecoveryReplicas {
		podIndex, err := stsobj.PodIndex(replica)
		if err != nil || podIndex == nil {
			continue
		}

		pvcState, ok := pvcStates[*podIndex]
		if !ok || pvcState.UID == "" || pvcState.CreationTimestamp.IsZero() {
			continue
		}
		if pvcState.CreationTimestamp.After(physicalBackup.CreationTimestamp.Time) {
			return true
		}
	}
	return false
}

func (r *MariaDBReconciler) reconcileReplicasToRecover(ctx context.Context, replicas []string, mariadb *mariadbv1alpha1.MariaDB,
	physicalBackup *mariadbv1alpha1.PhysicalBackup, snapshotKey *types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
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
			refreshed, refreshErr := r.retryReplicaRecoveryWithFreshBackup(ctx, replica, mariadb, err, replicaLogger)
			if refreshErr != nil {
				return ctrl.Result{}, refreshErr
			}
			if refreshed {
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}
			return ctrl.Result{}, fmt.Errorf("error ensuring replica %s configured: %v", replica, err)
		}
		if err := r.ensureReplicaRecovered(ctx, replica, mariadb, replicaLogger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring replica %s recovered: %v", replica, err)
		}
		podIndex, err := stsobj.PodIndex(replica)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
		}
		if err := r.syncStoragePVCUIDAnnotation(ctx, mariadb, *podIndex); err != nil {
			return ctrl.Result{}, fmt.Errorf("error syncing storage PVC annotation for replica %s: %v", replica, err)
		}
	}
	// Requeue to track replication status
	return ctrl.Result{Requeue: true}, nil
}

func (r *MariaDBReconciler) quiescePVCRecoveryReplicas(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replicas []string, logger logr.Logger) (ctrl.Result, error) {
	if len(replicas) == 0 {
		return ctrl.Result{}, nil
	}

	podsToDelete := make(map[string]types.NamespacedName, len(replicas))
	pendingRecoveryReplica := false
	for _, replica := range replicas {
		action, result, err := r.getPVCRecoveryQuiesceAction(ctx, mariadb, replica, logger)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !result.IsZero() {
			return result, nil
		}
		if !action.pending {
			continue
		}
		pendingRecoveryReplica = true
		if action.podKey != nil {
			podsToDelete[replica] = *action.podKey
		}
	}
	if !pendingRecoveryReplica {
		return ctrl.Result{}, nil
	}

	stsDeleted := false
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting StatefulSet: %v", err)
		}
	} else {
		if err := r.deleteStatefulSetLeavingOrphanPods(ctx, mariadb); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting StatefulSet: %v", err)
		}
		stsDeleted = true
	}

	podDeleted := false
	for replica, podKey := range podsToDelete {
		if err := r.ensurePodDeleted(ctx, podKey, logger.WithValues("replica", replica)); err != nil {
			return ctrl.Result{}, fmt.Errorf("error ensuring Pod deleted: %v", err)
		}
		podDeleted = true
	}

	if stsDeleted || podDeleted {
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

type pvcRecoveryQuiesceAction struct {
	pending bool
	podKey  *types.NamespacedName
}

func (r *MariaDBReconciler) getPVCRecoveryQuiesceAction(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replica string, logger logr.Logger) (pvcRecoveryQuiesceAction, ctrl.Result, error) {
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, fmt.Errorf("error getting replica pod index: %v", err)
	}
	pvcState, pvcStateFound, err := r.getStoragePVCState(ctx, mariadb, *podIndex)
	if err != nil {
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, fmt.Errorf("error getting replica storage PVC state: %v", err)
	}
	if pvcStateFound && isReplicaRecoveryCompletedForPVC(mariadb.Annotations, *podIndex, pvcState) {
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, nil
	}

	initJobComplete, err := r.isInitJobComplete(ctx, mariadb.PhysicalBackupInitJobKey(*podIndex))
	if err != nil {
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, fmt.Errorf("error checking recovery Job status: %v", err)
	}
	if initJobComplete {
		if pvcStateFound && pvcState.UID != "" {
			if err := r.recordReplicaRecoveryCompletedPVC(ctx, mariadb, *podIndex, pvcState.UID); err != nil {
				return pvcRecoveryQuiesceAction{}, ctrl.Result{}, err
			}
		}
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, nil
	}

	podKey := types.NamespacedName{
		Name:      replica,
		Namespace: mariadb.Namespace,
	}
	pod, err := r.getPodIfExists(ctx, podKey)
	if err != nil {
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, err
	}
	if pod == nil {
		if _, ok := storedReplicaRecoveryNode(mariadb.Annotations, *podIndex); !ok {
			logger.V(1).Info("Waiting for replica pod to exist before pausing PVC recovery target", "replica", replica)
			return pvcRecoveryQuiesceAction{}, ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		return pvcRecoveryQuiesceAction{pending: true}, ctrl.Result{}, nil
	}
	if pod.Spec.NodeName == "" {
		logger.V(1).Info("Waiting for replica pod to be scheduled before pausing PVC recovery target", "replica", replica)
		return pvcRecoveryQuiesceAction{}, ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if err := r.syncReplicaRecoveryNodeAnnotation(ctx, mariadb, *podIndex, pod.Spec.NodeName); err != nil {
		return pvcRecoveryQuiesceAction{}, ctrl.Result{}, fmt.Errorf("error recording replica recovery node: %v", err)
	}
	if mariadb.Annotations == nil {
		mariadb.Annotations = map[string]string{}
	}
	mariadb.Annotations[replicaRecoveryNodeAnnotationKey(*podIndex)] = pod.Spec.NodeName
	return pvcRecoveryQuiesceAction{
		pending: true,
		podKey:  &podKey,
	}, ctrl.Result{}, nil
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
	jobKey := mariadb.PhysicalBackupInitJobKey(*podIndex)

	pvcState, result, err := r.reconcileReplicaRecoveryPVC(ctx, mariadb, *podIndex, replica, logger)
	if !result.IsZero() || err != nil {
		return result, err
	}
	if result, err := r.ensureRecoveryJobCurrent(ctx, jobKey, pvcState, logger); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.setReplicaToRecover(ctx, mariadb, replica); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAndWaitForJobReplicaRecovery(
		ctx,
		physicalBackup,
		mariadb,
		*podIndex,
		podKey,
		jobKey,
		logger,
	); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.recordReplicaRecoveryCompletedPVC(ctx, mariadb, *podIndex, pvcState.UID); err != nil {
		return ctrl.Result{}, err
	}

	if result, err := r.reconcileStatefulSet(ctx, mariadb); !result.IsZero() || err != nil {
		return result, err
	}
	if result, err := r.waitForPodScheduled(ctx, mariadb, *podIndex, logger); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.setReplicaToRecover(ctx, mariadb, ""); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) recordReplicaRecoveryCompletedPVC(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int, pvcUID string) error {
	if err := r.syncReplicaRecoveryCompletedPVCUIDAnnotation(ctx, mariadb, podIndex, pvcUID); err != nil {
		return fmt.Errorf("error recording replica recovery completed PVC: %v", err)
	}
	if pvcUID == "" {
		return nil
	}
	if mariadb.Annotations == nil {
		mariadb.Annotations = map[string]string{}
	}
	mariadb.Annotations[replicaRecoveryCompletedPVCUIDAnnotationKey(podIndex)] = pvcUID
	return nil
}

func (r *MariaDBReconciler) reconcileReplicaRecoveryPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	podIndex int, replica string, logger logr.Logger) (storagePVCState, ctrl.Result, error) {
	if result, err := r.ensureStoragePVCPresent(ctx, mariadb, podIndex, logger); !result.IsZero() || err != nil {
		return storagePVCState{}, result, err
	}
	pvcState, ok, err := r.getStoragePVCState(ctx, mariadb, podIndex)
	if err != nil {
		return storagePVCState{}, ctrl.Result{}, fmt.Errorf("error getting storage PVC state: %v", err)
	}
	if !ok {
		return storagePVCState{}, ctrl.Result{}, fmt.Errorf("storage PVC state not found for replica %s", replica)
	}
	return pvcState, ctrl.Result{}, nil
}

func (r *MariaDBReconciler) ensureRecoveryJobCurrent(ctx context.Context, key types.NamespacedName,
	pvcState storagePVCState, logger logr.Logger) (ctrl.Result, error) {
	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting recovery Job: %v", err)
	}
	if !isInitJobStaleForPVC(&job, pvcState) {
		return ctrl.Result{}, nil
	}

	logger.Info(
		"Deleting stale PhysicalBackup init job for replaced storage PVC",
		"name", job.Name,
		"job-pvc-uid", job.Annotations[initJobStoragePVCUIDAnnotation],
		"current-pvc-uid", pvcState.UID,
	)
	if err := r.Delete(ctx, &job, &client.DeleteOptions{
		PropagationPolicy: ptr.To(metav1.DeletePropagationBackground),
	}); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("error deleting stale recovery Job: %v", err)
	}
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func (r *MariaDBReconciler) setReplicaToRecover(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, replica string) error {
	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if replica == "" {
			mariadb.SetReplicaToRecover(nil)
			return nil
		}
		mariadb.SetReplicaToRecover(&replica)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) reconcileAndWaitForJobReplicaRecovery(ctx context.Context, physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int, podKey, jobKey types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	recoveryJobComplete, err := r.isInitJobComplete(ctx, jobKey)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking recovery Job status: %v", err)
	}
	if recoveryJobComplete {
		return ctrl.Result{}, nil
	}

	pod, err := r.getPodIfExists(ctx, podKey)
	if err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.ensureRecoveryJobCreated(ctx, physicalBackup, mariadb, podIndex, pod, logger); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.deleteStatefulSetLeavingOrphanPods(ctx, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error deleting StatefulSet: %v", err)
	}
	if err := r.ensurePodDeleted(ctx, podKey, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error ensuring Pod deleted: %v", err)
	}
	return r.waitForInitJobComplete(ctx, mariadb, jobKey, logger)
}

func (r *MariaDBReconciler) getPodIfExists(ctx context.Context, key types.NamespacedName) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting Pod: %v", err)
	}
	return &pod, nil
}

func isInitJobStaleForPVC(job *batchv1.Job, pvcState storagePVCState) bool {
	if pvcState.UID == "" {
		return false
	}
	if jobPVCUID := job.Annotations[initJobStoragePVCUIDAnnotation]; jobPVCUID != "" {
		return jobPVCUID != pvcState.UID
	}
	if job.CreationTimestamp.IsZero() || pvcState.CreationTimestamp.IsZero() {
		return false
	}
	return job.CreationTimestamp.Time.Before(pvcState.CreationTimestamp.Time)
}

func (r *MariaDBReconciler) ensureRecoveryJobCreated(ctx context.Context, physicalBackup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int, pod *corev1.Pod, logger logr.Logger) (ctrl.Result, error) {
	jobKey := mariadb.PhysicalBackupInitJobKey(podIndex)

	var job batchv1.Job
	if err := r.Get(ctx, jobKey, &job); err == nil {
		return ctrl.Result{}, nil
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("error getting recovery Job: %v", err)
	}

	if pod == nil || pod.Spec.NodeName == "" {
		if nodeName, ok := storedReplicaRecoveryNode(mariadb.Annotations, podIndex); ok && nodeName != "" {
			logger.V(1).Info("Using stored recovery node to create PhysicalBackup init job", "node", nodeName)
			pod = &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeName: nodeName,
				},
			}
		} else if pod == nil {
			logger.V(1).Info("Waiting for Pod to exist before creating recovery Job")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		} else {
			logger.V(1).Info("Waiting for Pod to be scheduled before creating recovery Job", "pod", pod.Name)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	bootstrapFrom := ptr.Deref(replication.Replica.ReplicaBootstrapFrom, mariadbv1alpha1.ReplicaBootstrapFrom{})
	logger.Info("Creating PhysicalBackup init job", "name", jobKey.Name)
	if err := r.createInitJob(
		ctx,
		mariadb,
		jobKey,
		podIndex,
		builder.WithPhysicalBackup(
			physicalBackup,
			time.Now(),
			bootstrapFrom.RestoreJob,
			command.WithCleanupDataDir(true),
		),
		builder.WithReplicaRecovery(pod),
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating recovery Job: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) isInitJobComplete(ctx context.Context, key types.NamespacedName) (bool, error) {
	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return jobpkg.IsJobComplete(&job), nil
}

func (r *MariaDBReconciler) waitForInitJobComplete(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting recovery Job: %v", err)
	}
	if !jobpkg.IsJobComplete(&job) {
		schedulingErr, err := r.jobSchedulingError(ctx, key)
		if err != nil {
			return ctrl.Result{}, err
		}
		if schedulingErr != "" {
			errMsg := fmt.Sprintf("PhysicalBackup init Job '%s' is unschedulable: %s", key.Name, schedulingErr)
			if r.Recorder != nil {
				r.Recorder.Eventf(mariadb,
					nil,
					corev1.EventTypeWarning,
					mariadbv1alpha1.ReasonMariaDBReplicaRecoveryError,
					mariadbv1alpha1.ActionReconciling,
					"Unable to recover replicas: %s",
					errMsg,
				)
			}
			if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
				condition.SetReplicaRecoveryError(status, errMsg)
				return nil
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
			}
			logger.Info("Unable to recover replicas. Requeuing...", "err", errMsg)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.V(1).Info("PhysicalBackup init job not completed. Requeuing...")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
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

	if err := r.ensurePodDeleted(ctx, podKey, logger); err != nil {
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

func (r *MariaDBReconciler) ensurePodDeleted(ctx context.Context, key types.NamespacedName, logger logr.Logger) error {
	deletePodCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	return wait.PollUntilSuccessOrContextCancelWithInterval(deletePodCtx, 5*time.Second, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, key, &pod); err != nil {
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

		replStatus, err := client.ReplicaStatus(ctx, logger)
		if err != nil {
			return fmt.Errorf("error getting replica status: %v", err)
		}

		if replStatus.LastIOErrno != nil && *replStatus.LastIOErrno == 0 &&
			replStatus.LastSQLErrno != nil && *replStatus.LastSQLErrno == 0 {
			logger.Info("Replica recovered")
			return nil
		}
		return errors.New("replica not recovered")
	})
}

func (r *MariaDBReconciler) retryReplicaRecoveryWithFreshBackup(ctx context.Context, replica string,
	mariadb *mariadbv1alpha1.MariaDB, replicationErr error, logger logr.Logger) (bool, error) {
	if !errors.Is(replicationErr, context.DeadlineExceeded) {
		return false, nil
	}

	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return false, fmt.Errorf("error getting replica pod index: %v", err)
	}

	pvcState, ok, err := r.getStoragePVCState(ctx, mariadb, *podIndex)
	if err != nil {
		return false, fmt.Errorf("error getting storage PVC state: %v", err)
	}
	if !ok || pvcState.UID == "" {
		return false, nil
	}

	if refreshedUID, ok := storedReplicaRecoveryRefreshPVCUID(mariadb.Annotations, *podIndex); ok && refreshedUID == pvcState.UID {
		logger.Info(
			"Replica recovery already retried with a fresh PhysicalBackup for this PVC",
			"replica-pvc-uid", pvcState.UID,
		)
		return false, nil
	}

	if err := r.syncReplicaRecoveryRefreshPVCUIDAnnotation(ctx, mariadb, *podIndex, pvcState.UID); err != nil {
		return false, fmt.Errorf("error recording replica recovery retry: %v", err)
	}
	if err := r.clearReplicaRecoveryCompletedPVCUIDAnnotation(ctx, mariadb, *podIndex); err != nil {
		return false, fmt.Errorf("error clearing replica recovery completed PVC annotation: %v", err)
	}
	if err := r.cleanupReplicaRecoveryArtifacts(ctx, mariadb); err != nil {
		return false, fmt.Errorf("error cleaning replica recovery artifacts: %v", err)
	}

	logger.Info(
		"Replica recovery configuration timed out, retrying with a fresh PhysicalBackup",
		"replica-pvc-uid", pvcState.UID,
	)
	return true, nil
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

	if err := r.cleanupReplicaRecoveryArtifacts(ctx, mariadb); err != nil {
		return err
	}
	if err := r.clearReplicaRecoveryRefreshPVCUIDAnnotations(ctx, mariadb); err != nil {
		return fmt.Errorf("error clearing replica recovery retry annotations: %v", err)
	}
	if err := r.clearReplicaRecoveryNodeAnnotations(ctx, mariadb); err != nil {
		return fmt.Errorf("error clearing replica recovery node annotations: %v", err)
	}
	if err := r.clearReplicaRecoveryCompletedPVCUIDAnnotations(ctx, mariadb); err != nil {
		return fmt.Errorf("error clearing replica recovery completed PVC annotations: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) cleanupReplicaRecoveryArtifacts(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) error {
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
	for replica, err := range replication.Replicas {
		if isRecoverableError(
			mdb,
			err,
			recoverableIOErrorCodes,
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

func getReplicasWithImmediateRecoverableErrors(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) []string {
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	var replicas []string
	for replica, status := range replication.Replicas {
		if hasImmediateRecoverableError(
			status,
			recoverableIOErrorCodes,
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

func mergeReplicasToRecover(replicaSets ...[]string) []string {
	replicasSet := make(map[string]struct{})
	for _, replicas := range replicaSets {
		for _, replica := range replicas {
			replicasSet[replica] = struct{}{}
		}
	}

	replicas := make([]string, 0, len(replicasSet))
	for replica := range replicasSet {
		replicas = append(replicas, replica)
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] < replicas[j]
	})
	return replicas
}

func hasImmediateRecoverableError(status mariadbv1alpha1.ReplicaStatus,
	recoverableIOErrorCodes []int, logger logr.Logger) bool {
	for _, code := range recoverableIOErrorCodes {
		if status.LastIOErrno != nil && *status.LastIOErrno == code {
			logger.V(1).Info("Recoverable IO error code detected", "io-errno", *status.LastIOErrno)
			return true
		}
	}
	return false
}

func isRecoverableError(mdb *mariadbv1alpha1.MariaDB, status mariadbv1alpha1.ReplicaStatus,
	recoverableIOErrorCodes []int, logger logr.Logger) bool {
	if hasImmediateRecoverableError(status, recoverableIOErrorCodes, logger) {
		return true
	}
	lastIOErrno := ptr.Deref(status.LastIOErrno, 0)
	lastSQLErrno := ptr.Deref(status.LastSQLErrno, 0)

	if (lastIOErrno != 0 || lastSQLErrno != 0) && !status.LastErrorTransitionTime.IsZero() {
		replication := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
		recovery := ptr.Deref(replication.Replica.ReplicaRecovery, mariadbv1alpha1.ReplicaRecovery{})
		errThreshold := ptr.Deref(recovery.ErrorDurationThreshold, metav1.Duration{Duration: 5 * time.Minute})
		age := time.Since(status.LastErrorTransitionTime.Time)

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
