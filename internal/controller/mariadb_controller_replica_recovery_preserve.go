package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	replicaRecoveryPreserveLabel         = "k8s.mariadb.com/replica-recovery-preserve"
	replicaRecoveryPreservePodIndexLabel = "k8s.mariadb.com/replica-recovery-pod-index"
	replicaRecoveryPreserveIDLabel       = "k8s.mariadb.com/replica-recovery-id"

	replicaRecoveryPreservedAnnotationPrefix = "k8s.mariadb.com/replica-recovery-preserved-"

	replicaRecoveryPreserveKeep          = 3
	replicaRecoveryPreserveMaxAge        = 7 * 24 * time.Hour
	replicaRecoveryPreservePollDelay     = 5 * time.Second
	replicaRecoveryPreserveRetryDelay    = 30 * time.Second
	replicaRecoveryPreserveRefusedDelay  = time.Minute
	replicaRecoveryPreserveSQLTimeout    = 5 * time.Second
	replicaRecoveryPreserveTimestampForm = "20060102150405"
)

func (r *MariaDBReconciler) preserveReplicasBeforeRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replicas []string, logger logr.Logger) (ctrl.Result, error) {
	for _, replica := range replicas {
		result, err := r.preserveReplicaBeforeRecovery(ctx, mariadb, replica,
			logger.WithValues("replica", replica).WithName("preserve"))
		if !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

// preserveReplicaBeforeRecovery keeps the data of a replica that is about to be wiped by the recovery process.
// A replica in error is frequently a demoted primary, and a demoted primary can hold transactions the new primary
// never received. With a preserve VolumeSnapshotClass configured, the replica storage is snapshotted and the recovery
// goes on once the snapshot is ready. Without one, the replica is only recovered when its binary log proves that the
// primary already holds every transaction the replica wrote itself. Both run before the recovery stops the replica
// Pod, and the outcome is recorded for the recovery episode: re-checking later found the Pod already gone and
// refused every recovery without a snapshot class.
func (r *MariaDBReconciler) preserveReplicaBeforeRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replica string, logger logr.Logger) (ctrl.Result, error) {
	podIndex, err := stsobj.PodIndex(replica)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting replica Pod index: %v", err)
	}
	pvc, wiped, err := r.replicaStorageToPreserve(ctx, mariadb, *podIndex)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pvc == nil || wiped {
		return ctrl.Result{}, nil
	}

	recoveryID := replicaRecoveryPreserveEpisode(mariadb, pvc)
	if mariadb.Annotations[replicaRecoveryPreservedAnnotationKey(*podIndex)] == recoveryID {
		return ctrl.Result{}, nil
	}
	snapshots, err := r.listReplicaRecoveryPreserveSnapshots(ctx, mariadb, *podIndex)
	if err != nil {
		return ctrl.Result{}, err
	}
	for i := range snapshots {
		snapshot := &snapshots[i]
		if snapshot.Labels[replicaRecoveryPreserveIDLabel] != recoveryID {
			continue
		}
		result, err := r.waitForReplicaRecoveryPreserveSnapshot(ctx, mariadb, snapshot, snapshots, logger)
		if !result.IsZero() || err != nil {
			return result, err
		}
		return ctrl.Result{}, r.recordReplicaRecoveryPreserved(ctx, mariadb, *podIndex, recoveryID)
	}

	className := preserveVolumeSnapshotClassName(mariadb)
	if className == "" {
		result, err := r.verifyReplicaHoldsNoUniqueTransactions(ctx, mariadb, replica, *podIndex, logger)
		if !result.IsZero() || err != nil {
			return result, err
		}
		return ctrl.Result{}, r.recordReplicaRecoveryPreserved(ctx, mariadb, *podIndex, recoveryID)
	}

	snapshot := buildReplicaRecoveryPreserveSnapshot(mariadb, pvc, *podIndex, recoveryID, className, time.Now())
	if err := r.Create(ctx, snapshot); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("error creating replica recovery preserve VolumeSnapshot: %v", err)
	}
	logger.Info("Preserving replica storage before recovery", "volume-snapshot", snapshot.Name, "pvc", pvc.Name)
	if r.Recorder != nil {
		r.Recorder.Eventf(mariadb,
			nil,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonMariaDBReplicaRecoveryPreserved,
			mariadbv1alpha1.ActionReconciling,
			"Preserving storage of replica %s in VolumeSnapshot %s before recovering it", replica, snapshot.Name,
		)
	}
	return ctrl.Result{RequeueAfter: replicaRecoveryPreservePollDelay}, nil
}

// replicaRecoveryPreserveEpisode identifies one recovery of one replica storage generation: the PVC UID plus the
// time the ReplicaRecovered condition turned false, which stays fixed until the recovery ends.
func replicaRecoveryPreserveEpisode(mariadb *mariadbv1alpha1.MariaDB, pvc *corev1.PersistentVolumeClaim) string {
	recovering := meta.FindStatusCondition(mariadb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered)
	if recovering == nil || recovering.Status != metav1.ConditionFalse || recovering.LastTransitionTime.IsZero() {
		return string(pvc.UID)
	}
	return fmt.Sprintf("%s-%d", pvc.UID, recovering.LastTransitionTime.Unix())
}

func replicaRecoveryPreservedAnnotationKey(podIndex int) string {
	return fmt.Sprintf("%s%d", replicaRecoveryPreservedAnnotationPrefix, podIndex)
}

func (r *MariaDBReconciler) recordReplicaRecoveryPreserved(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	podIndex int, recoveryID string) error {
	if err := r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) error {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[replicaRecoveryPreservedAnnotationKey(podIndex)] = recoveryID
		return nil
	}); err != nil {
		return fmt.Errorf("error recording replica recovery preserve: %v", err)
	}
	return nil
}

// replicaStorageToPreserve returns the storage PVC that still holds the replica data. A missing or recreated PVC
// holds nothing to preserve, and a recovery Job that already exists means the data has been or is being replaced.
func (r *MariaDBReconciler) replicaStorageToPreserve(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	podIndex int) (*corev1.PersistentVolumeClaim, bool, error) {
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, mariadb.PVCKey(builder.StorageVolume, podIndex), &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("error getting replica storage PVC: %v", err)
	}
	if storedUID, ok := storedStoragePVCUID(mariadb.Annotations, podIndex); ok && storedUID != "" &&
		storedUID != string(pvc.UID) {
		return nil, false, nil
	}
	var initJob batchv1.Job
	if err := r.Get(ctx, mariadb.PhysicalBackupInitJobKey(podIndex), &initJob); err == nil {
		return &pvc, true, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, false, fmt.Errorf("error getting recovery Job: %v", err)
	}
	return &pvc, false, nil
}

func (r *MariaDBReconciler) waitForReplicaRecoveryPreserveSnapshot(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	snapshot *volumesnapshotv1.VolumeSnapshot, snapshots []volumesnapshotv1.VolumeSnapshot,
	logger logr.Logger) (ctrl.Result, error) {
	status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
	if status.Error != nil && status.Error.Message != nil {
		logger.Info("Replica recovery preserve VolumeSnapshot failed. Retrying...",
			"volume-snapshot", snapshot.Name, "err", *status.Error.Message)
		if r.Recorder != nil {
			r.Recorder.Eventf(mariadb,
				nil,
				corev1.EventTypeWarning,
				mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused,
				mariadbv1alpha1.ActionReconciling,
				"Not recovering yet: preserve VolumeSnapshot %s failed: %s", snapshot.Name, *status.Error.Message,
			)
		}
		return ctrl.Result{RequeueAfter: replicaRecoveryPreserveRetryDelay}, nil
	}
	if !ptr.Deref(status.ReadyToUse, false) {
		logger.V(1).Info("Waiting for replica recovery preserve VolumeSnapshot", "volume-snapshot", snapshot.Name)
		return ctrl.Result{RequeueAfter: replicaRecoveryPreservePollDelay}, nil
	}
	if err := r.pruneReplicaRecoveryPreserveSnapshots(ctx, snapshots, logger); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// pruneReplicaRecoveryPreserveSnapshots keeps the newest preserve snapshots of a replica and deletes the rest.
func (r *MariaDBReconciler) pruneReplicaRecoveryPreserveSnapshots(ctx context.Context,
	snapshots []volumesnapshotv1.VolumeSnapshot, logger logr.Logger) error {
	for _, snapshot := range replicaRecoveryPreserveSnapshotsToPrune(snapshots, replicaRecoveryPreserveKeep) {
		logger.Info("Deleting old replica recovery preserve VolumeSnapshot", "volume-snapshot", snapshot.Name)
		if err := r.Delete(ctx, &snapshot); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting replica recovery preserve VolumeSnapshot %s: %v", snapshot.Name, err)
		}
	}
	return nil
}

func replicaRecoveryPreserveSnapshotsToPrune(snapshots []volumesnapshotv1.VolumeSnapshot,
	keep int) []volumesnapshotv1.VolumeSnapshot {
	ready := make([]volumesnapshotv1.VolumeSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if ptr.Deref(ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{}).ReadyToUse, false) {
			ready = append(ready, snapshot)
		}
	}
	if len(ready) <= keep {
		return nil
	}
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].CreationTimestamp.Equal(&ready[j].CreationTimestamp) {
			return ready[i].Name > ready[j].Name
		}
		return ready[j].CreationTimestamp.Before(&ready[i].CreationTimestamp)
	})
	return ready[keep:]
}

// expireReplicaRecoveryPreserveSnapshots deletes every preserve snapshot of the MariaDB older than
// replicaRecoveryPreserveMaxAge, the newest ones included, so that no copy of a replica outlives it. It returns how
// long until the next remaining snapshot expires, or zero when none remains. A cluster without the VolumeSnapshot
// API holds no snapshots to expire.
func (r *MariaDBReconciler) expireReplicaRecoveryPreserveSnapshots(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, now time.Time, logger logr.Logger) (time.Duration, error) {
	var list volumesnapshotv1.VolumeSnapshotList
	if err := r.List(ctx, &list,
		client.InNamespace(mariadb.Namespace),
		client.MatchingLabels{replicaRecoveryPreserveLabel: mariadb.Name},
	); err != nil {
		if meta.IsNoMatchError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("error listing replica recovery preserve VolumeSnapshots: %v", err)
	}
	var next time.Duration
	for i := range list.Items {
		snapshot := &list.Items[i]
		remaining := snapshot.CreationTimestamp.Add(replicaRecoveryPreserveMaxAge).Sub(now)
		if remaining > 0 {
			if next == 0 || remaining < next {
				next = remaining
			}
			continue
		}
		logger.Info("Deleting expired replica recovery preserve VolumeSnapshot", "volume-snapshot", snapshot.Name,
			"created", snapshot.CreationTimestamp.UTC())
		if err := r.Delete(ctx, snapshot); err != nil && !apierrors.IsNotFound(err) {
			return 0, fmt.Errorf("error deleting replica recovery preserve VolumeSnapshot %s: %v", snapshot.Name, err)
		}
	}
	return next, nil
}

func (r *MariaDBReconciler) listReplicaRecoveryPreserveSnapshots(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	podIndex int) ([]volumesnapshotv1.VolumeSnapshot, error) {
	var list volumesnapshotv1.VolumeSnapshotList
	if err := r.List(ctx, &list,
		client.InNamespace(mariadb.Namespace),
		client.MatchingLabels{
			replicaRecoveryPreserveLabel:         mariadb.Name,
			replicaRecoveryPreservePodIndexLabel: strconv.Itoa(podIndex),
		},
	); err != nil {
		return nil, fmt.Errorf("error listing replica recovery preserve VolumeSnapshots: %v", err)
	}
	return list.Items, nil
}

func buildReplicaRecoveryPreserveSnapshot(mariadb *mariadbv1alpha1.MariaDB, pvc *corev1.PersistentVolumeClaim,
	podIndex int, recoveryID, className string, now time.Time) *volumesnapshotv1.VolumeSnapshot {
	return &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-preserve-%d-%s", mariadb.Name, podIndex,
				now.UTC().Format(replicaRecoveryPreserveTimestampForm)),
			Namespace: mariadb.Namespace,
			Labels: map[string]string{
				replicaRecoveryPreserveLabel:         mariadb.Name,
				replicaRecoveryPreservePodIndexLabel: strconv.Itoa(podIndex),
				replicaRecoveryPreserveIDLabel:       recoveryID,
			},
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			Source: volumesnapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: ptr.To(pvc.Name),
			},
			VolumeSnapshotClassName: ptr.To(className),
		},
	}
}

func preserveVolumeSnapshotClassName(mariadb *mariadbv1alpha1.MariaDB) string {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	recovery := ptr.Deref(replication.Replica.ReplicaRecovery, mariadbv1alpha1.ReplicaRecovery{})
	return ptr.Deref(recovery.PreserveVolumeSnapshotClassName, "")
}

// verifyReplicaHoldsNoUniqueTransactions lets a recovery without preserve snapshots go on only when the replica
// provably holds nothing the primary lacks. Any doubt, including an unreachable replica, refuses the recovery.
func (r *MariaDBReconciler) verifyReplicaHoldsNoUniqueTransactions(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replica string, podIndex int, logger logr.Logger) (ctrl.Result, error) {
	unique, err := r.replicaHoldsUniqueTransactions(ctx, mariadb, podIndex)
	if err == nil && !unique {
		return ctrl.Result{}, nil
	}
	reason := "it may hold transactions the primary lacks"
	if err != nil {
		reason = fmt.Sprintf("its transactions could not be compared with the primary: %v", err)
	}
	logger.Info("Refusing to recover replica without a preserve VolumeSnapshotClass", "reason", reason)
	if r.Recorder != nil {
		r.Recorder.Eventf(mariadb,
			nil,
			corev1.EventTypeWarning,
			mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused,
			mariadbv1alpha1.ActionReconciling,
			"Refusing to recover replica %s: %s. Set spec.replication.replica.recovery.preserveVolumeSnapshotClassName "+
				"to snapshot it before recovery", replica, reason,
		)
	}
	return ctrl.Result{RequeueAfter: replicaRecoveryPreserveRefusedDelay}, nil
}

func (r *MariaDBReconciler) replicaHoldsUniqueTransactions(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	podIndex int) (bool, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return false, errors.New("current primary is unknown")
	}
	replicaClient, err := sql.NewInternalClientWithPodIndex(ctx, mariadb, r.RefResolver, podIndex,
		sql.WithTimeout(replicaRecoveryPreserveSQLTimeout))
	if err != nil {
		return false, fmt.Errorf("error connecting to replica: %v", err)
	}
	defer replicaClient.Close()
	primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, mariadb, r.RefResolver,
		*mariadb.Status.CurrentPrimaryPodIndex, sql.WithTimeout(replicaRecoveryPreserveSQLTimeout))
	if err != nil {
		return false, fmt.Errorf("error connecting to primary: %v", err)
	}
	defer primaryClient.Close()

	rawServerID, err := replicaClient.SystemVariable(ctx, "server_id")
	if err != nil {
		return false, fmt.Errorf("error getting replica server_id: %v", err)
	}
	serverID, err := strconv.ParseUint(rawServerID, 10, 32)
	if err != nil {
		return false, fmt.Errorf("error parsing replica server_id %q: %v", rawServerID, err)
	}
	replicaBinlogPos, err := replicaClient.SystemVariable(ctx, "gtid_binlog_pos")
	if err != nil {
		return false, fmt.Errorf("error getting replica gtid_binlog_pos: %v", err)
	}
	primaryBinlogState, err := primaryClient.SystemVariable(ctx, "gtid_binlog_state")
	if err != nil {
		return false, fmt.Errorf("error getting primary gtid_binlog_state: %v", err)
	}
	primarySlavePos, err := primaryClient.SystemVariable(ctx, "gtid_slave_pos")
	if err != nil {
		return false, fmt.Errorf("error getting primary gtid_slave_pos: %v", err)
	}
	return holdsUnreplicatedOwnTransactions(uint32(serverID), replicaBinlogPos, primaryBinlogState, primarySlavePos)
}

// holdsUnreplicatedOwnTransactions reports whether a node wrote transactions that the primary does not hold.
// The node's own writes are the GTIDs carrying its server_id in its gtid_binlog_pos. The primary holds such a GTID
// when its gtid_binlog_state or gtid_slave_pos records the same domain and server_id with an equal or higher
// sequence number, which under gtid_strict_mode implies every earlier transaction of that server.
func holdsUnreplicatedOwnTransactions(serverID uint32, nodeBinlogPos, primaryBinlogState,
	primarySlavePos string) (bool, error) {
	nodeGtids, err := parseGtidList(nodeBinlogPos)
	if err != nil {
		return false, fmt.Errorf("error parsing node gtid_binlog_pos: %v", err)
	}
	primaryGtids, err := parseGtidList(primaryBinlogState + "," + primarySlavePos)
	if err != nil {
		return false, fmt.Errorf("error parsing primary GTID state: %v", err)
	}
	for _, own := range nodeGtids {
		if own.ServerID != serverID {
			continue
		}
		covered := false
		for _, held := range primaryGtids {
			if held.DomainID == own.DomainID && held.ServerID == own.ServerID && held.SequenceID >= own.SequenceID {
				covered = true
				break
			}
		}
		if !covered {
			return true, nil
		}
	}
	return false, nil
}

func parseGtidList(raw string) ([]*replication.Gtid, error) {
	var gtids []*replication.Gtid
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		gtid, err := replication.ParseGtid(part)
		if err != nil {
			return nil, err
		}
		gtids = append(gtids, gtid)
	}
	return gtids, nil
}
