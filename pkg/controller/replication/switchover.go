package replication

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/v26/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/wait"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

type switchoverPhase struct {
	name      string
	reconcile func(context.Context, *ReconcileRequest, logr.Logger) error
}

func isSwitchoverStale(mdb *mariadbv1alpha1.MariaDB) bool {
	return mdb.IsSwitchingPrimary() && !mdb.IsReplicationSwitchoverRequired()
}

// switchoverCandidate returns the Pod that spec.replication.primary.podIndex designates as the new primary.
// It is only meaningful when IsReplicationSwitchoverRequired holds, which guarantees the index is set.
func switchoverCandidate(mdb *mariadbv1alpha1.MariaDB) string {
	replication := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
	return statefulset.PodName(mdb.ObjectMeta, ptr.Deref(replication.Primary.PodIndex, 0))
}

func isSwitchoverAllowed(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsMaxScaleEnabled() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() {
		return false
	}
	return mdb.IsReplicationSwitchoverRequired()
}

func shouldReconcileSwitchover(mdb *mariadbv1alpha1.MariaDB) bool {
	if !isSwitchoverAllowed(mdb) {
		return false
	}
	// An in-flight switchover is always resumed. Its "Configure new primary" phase promotes the candidate,
	// so a two-node cluster has no configured replica left mid-flight: gating the resume on one left the old
	// primary read-locked and read-only forever, while the promoted candidate accepted writes that the
	// status never acknowledged.
	if mdb.IsSwitchingPrimary() {
		return true
	}
	// A switchover only starts towards a configured replica. The phases need a replica to sync and promote;
	// a candidate that is still being recovered, or that was never configured, would otherwise lock the
	// primary with nothing to switch to.
	return mdb.IsConfiguredReplica(switchoverCandidate(mdb))
}

// isSwitchoverPostponed reports whether a required switchover is waiting for its new primary to become a
// configured replica.
func isSwitchoverPostponed(mdb *mariadbv1alpha1.MariaDB) bool {
	return isSwitchoverAllowed(mdb) && !mdb.IsSwitchingPrimary() && !mdb.IsConfiguredReplica(switchoverCandidate(mdb))
}

// switchoverAbandonReason returns why an in-flight switchover can never complete, or an empty string while it
// still can. The new primary's Pod must exist and must not be under replica recovery: a recovery wipes the
// candidate's data, so there is nothing left to promote, and the recovery cannot finish while the switchover
// holds the current primary locked.
func switchoverAbandonReason(mdb *mariadbv1alpha1.MariaDB, candidatePodExists bool) string {
	if !mdb.IsSwitchingPrimary() || !mdb.IsReplicationSwitchoverRequired() {
		return ""
	}
	candidate := switchoverCandidate(mdb)
	if !candidatePodExists {
		return fmt.Sprintf("Pod '%s' does not exist", candidate)
	}
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	if ptr.Deref(replication.ReplicaToRecover, "") == candidate {
		return fmt.Sprintf("Pod '%s' is under replica recovery", candidate)
	}
	return ""
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *ReconcileRequest, switchoverLogger logr.Logger) error {
	logger := switchoverLogger.WithValues("mariadb", req.mariadb.Name)

	currentPrimaryReady, err := r.currentPrimaryReady(ctx, req.mariadb, req.replClientSet)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	req.currentPrimaryReady = currentPrimaryReady

	if err := r.reconcileStaleSwitchover(ctx, req, logger); err != nil {
		return fmt.Errorf("error reconciling stale switchover: %v", err)
	}
	if err := r.reconcileAbandonedSwitchover(ctx, req, logger); err != nil {
		return fmt.Errorf("error reconciling abandoned switchover: %v", err)
	}
	if !shouldReconcileSwitchover(req.mariadb) {
		r.recordPostponedSwitchover(req, logger)
		return nil
	}

	replication := ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	primary := req.mariadb.Status.CurrentPrimaryPodIndex
	newPrimary := *replication.Primary.PodIndex
	newPrimaryPodName := statefulset.PodName(req.mariadb.ObjectMeta, *replication.Primary.PodIndex)
	logger = logger.WithValues("primary", primary, "new-primary", newPrimary)

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		condition.SetPrimarySwitching(&req.mariadb.Status, newPrimaryPodName)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}

	phases := []switchoverPhase{
		{
			name:      "Lock primary with read lock",
			reconcile: r.lockPrimaryWithReadLock,
		},
		{
			name:      "Set read_only in primary",
			reconcile: r.setPrimaryReadOnly,
		},
		{
			name:      "Wait sync",
			reconcile: r.waitSync,
		},
		{
			name:      "Configure new primary",
			reconcile: r.configureNewPrimary,
		},
		{
			name:      "Connect replicas to new primary",
			reconcile: r.connectReplicasToNewPrimary,
		},
		{
			name:      "Change primary to replica",
			reconcile: r.changePrimaryToReplica,
		},
	}

	for _, p := range phases {
		if err := p.reconcile(ctx, req, logger); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error in %s switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.UpdateCurrentPrimary(req.mariadb, newPrimary)
		condition.SetPrimarySwitched(&req.mariadb.Status)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}

	logger.Info("Primary switched")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitched,
		mariadbv1alpha1.ActionReconciling, "Primary switched from index '%d' to index '%d'", *primary, newPrimary)
	return nil
}

func (r *ReplicationReconciler) reconcileStaleSwitchover(ctx context.Context, req *ReconcileRequest,
	logger logr.Logger) error {
	if !isSwitchoverStale(req.mariadb) {
		return nil
	}
	if !req.currentPrimaryReady {
		logger.Info("Skipped stale switchover reconciliation due to primary's non ready status")
		return nil
	}
	if err := r.resetSwitchover(ctx, req, logger); err != nil {
		return err
	}

	logger.Info("Stale switchover has been reset")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationResetStaleSwitchover,
		mariadbv1alpha1.ActionReconciling, "Stale switchover has been reset")
	return nil
}

// reconcileAbandonedSwitchover resets an in-flight switchover whose new primary can no longer be promoted,
// returning the current primary to service. The switchover is not retried until the candidate is a
// configured replica again, see shouldReconcileSwitchover.
func (r *ReplicationReconciler) reconcileAbandonedSwitchover(ctx context.Context, req *ReconcileRequest,
	logger logr.Logger) error {
	if !req.mariadb.IsSwitchingPrimary() || !req.mariadb.IsReplicationSwitchoverRequired() {
		return nil
	}
	candidate := switchoverCandidate(req.mariadb)
	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Name: candidate, Namespace: req.mariadb.Namespace}, &pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting Pod '%s': %v", candidate, err)
	}
	reason := switchoverAbandonReason(req.mariadb, err == nil)
	if reason == "" {
		return nil
	}
	if !req.currentPrimaryReady {
		logger.Info("Skipped abandoning switchover due to primary's non ready status", "new-primary", candidate, "reason", reason)
		return nil
	}
	logger.Info("Abandoning switchover", "new-primary", candidate, "reason", reason)
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationSwitchoverAbandoned,
		mariadbv1alpha1.ActionReconciling, "Switchover to '%s' abandoned: %s", candidate, reason)
	return r.resetSwitchover(ctx, req, logger)
}

func (r *ReplicationReconciler) recordPostponedSwitchover(req *ReconcileRequest, logger logr.Logger) {
	if !isSwitchoverPostponed(req.mariadb) {
		return
	}
	candidate := switchoverCandidate(req.mariadb)
	logger.Info("Switchover postponed until the new primary is a configured replica", "new-primary", candidate)
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationSwitchoverPostponed,
		mariadbv1alpha1.ActionReconciling, "Switchover to '%s' postponed: Pod is not a configured replica", candidate)
}

// resetSwitchover unlocks the current primary, makes it writable again and clears the switching condition.
func (r *ReplicationReconciler) resetSwitchover(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	currentPrimaryClient, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	logger.Info("Unlocking primary")
	if err := currentPrimaryClient.UnlockTables(ctx); err != nil {
		return fmt.Errorf("error unlocking primary: %v", err)
	}

	logger.Info("Disabling readonly in primary")
	if err := currentPrimaryClient.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling readonly in primary: %v", err)
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		condition.SetPrimarySwitched(&req.mariadb.Status)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) lockPrimaryWithReadLock(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	if !req.currentPrimaryReady {
		logger.Info("Skipped locking primary with read lock due to primary's non ready status")
		return nil
	}
	client, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	logger.Info("Locking primary with read lock")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryLock,
		mariadbv1alpha1.ActionReconciling, "Locking primary with read lock")
	return client.LockTablesWithReadLock(ctx)
}

func (r *ReplicationReconciler) setPrimaryReadOnly(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	if !req.currentPrimaryReady {
		logger.Info("Skipped enabling readonly mode in primary due to primary's non ready status")
		return nil
	}
	client, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	logger.Info("Enabling readonly mode in primary")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryReadonly,
		mariadbv1alpha1.ActionReconciling, "Enabling readonly mode in primary")
	return client.EnableReadOnly(ctx)
}

func (r *ReplicationReconciler) waitSync(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	if req.currentPrimaryReady {
		return r.waitForReplicaSync(ctx, req, logger)
	}
	return r.waitForNewPrimarySync(ctx, req, logger)
}

func (r *ReplicationReconciler) waitForReplicaSync(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	if !req.currentPrimaryReady {
		logger.Info("Skipped waiting for replicas to be synced with primary due to primary's non ready status")
		return nil
	}

	primaryClient, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}
	primaryGtid, err := primaryClient.GtidBinlogPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting primary GTID binlog pos: %v", err)
	}
	if primaryGtid == "" {
		return errors.New("primary GTID (gtid_binlog_pos) is empty")
	}

	logger.Info("Waiting for replicas to be synced with primary", "gtid", primaryGtid)
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationReplicaSync,
		mariadbv1alpha1.ActionReconciling, "Waiting for replicas to be synced with primary")
	replication := ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{})

	g := new(errgroup.Group)
	g.SetLimit(int(req.mariadb.Spec.Replicas))

	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i == *req.mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		g.Go(func() error {
			replClient, err := req.replClientSet.clientForIndex(ctx, i)
			if err != nil {
				return fmt.Errorf("error getting replica '%d' client: %v", i, err)
			}
			logger.V(1).Info("Syncing replica with primary GTID", "replica", i, "gtid", primaryGtid)
			syncTimeout := ptr.Deref(replication.Replica.SyncTimeout, metav1.Duration{Duration: 10 * time.Second}).Duration

			if err := replClient.WaitForReplicaGtid(ctx, primaryGtid, syncTimeout); err != nil {
				logger.Error(err, "Error waiting for GTID in replica", "gtid", primaryGtid, "replica", i)
				r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationReplicaSyncErr,
					mariadbv1alpha1.ActionReconciling, "Error waiting for GTID '%s' in replica '%d': %v", primaryGtid, i, err)
				return err
			}

			logger.V(1).Info("Replica synced", "replica", i, "gtid", primaryGtid)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error waiting for replica sync: %w", err)
	}

	req.replicasSynced = true
	return nil
}

func (r *ReplicationReconciler) waitForNewPrimarySync(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	replication := ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	newPrimaryClient, err := req.replClientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	logger.Info("Waiting for new primary to be synced")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryNewSync,
		mariadbv1alpha1.ActionReconciling, "Waiting for new primary to be synced")

	syncTimeout := ptr.Deref(replication.Replica.SyncTimeout, metav1.Duration{Duration: 10 * time.Second}).Duration
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	if err := wait.PollUntilSuccessOrContextCancel(syncCtx, logger, func(ctx context.Context) error {
		status, err := newPrimaryClient.ReplicaStatus(ctx, logger)
		if err != nil {
			return fmt.Errorf("error getting new primary status: %v", err)
		}
		gtidDomainId, err := newPrimaryClient.GtidDomainId(ctx)
		if err != nil {
			return fmt.Errorf("error getting GTID domain ID in new primary: %v", err)
		}
		hasRelayLogEvents, err := HasRelayLogEvents(status, *gtidDomainId, logger)
		if err != nil {
			return fmt.Errorf("error checking relay logs: %v", err)
		}
		if hasRelayLogEvents {
			return errors.New("relay log events detected")
		}
		return nil
	}); err != nil {
		logger.Error(err, "Error waiting for new primary to be synced")
		r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationPrimaryNewSyncErr,
			mariadbv1alpha1.ActionReconciling, "Error waiting for new primary to be synced: %v", err)
		return err
	}

	logger.V(1).Info("New primary synced")
	return nil
}

func (r *ReplicationReconciler) configureNewPrimary(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	newPrimary := *ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{}).Primary.PodIndex
	newPrimaryClient, err := req.replClientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	logger.Info("Configuring new primary")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryNew,
		mariadbv1alpha1.ActionReconciling, "Configuring new primary at index '%d'", newPrimary)

	if err := r.replConfigClient.ConfigurePrimary(ctx, req.mariadb, newPrimaryClient); err != nil {
		return fmt.Errorf("error configuring new primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) connectReplicasToNewPrimary(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}

	newPrimary := *ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{}).Primary.PodIndex
	newPrimaryClient, err := req.replClientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	logger.Info("Connecting replicas to new primary")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationReplicaConn,
		mariadbv1alpha1.ActionReconciling, "Connecting replicas to new primary at '%d'", newPrimary)

	replicaOpts, err := r.configureReplicaOpts(ctx, req, newPrimaryClient, logger)
	if err != nil {
		return fmt.Errorf("error getting replica options: %v", err)
	}

	replicationPrimaryPodIndex := ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{}).Primary.PodIndex

	g := new(errgroup.Group)
	g.SetLimit(int(req.mariadb.Spec.Replicas))

	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		if i == *req.mariadb.Status.CurrentPrimaryPodIndex || i == *replicationPrimaryPodIndex {
			continue
		}
		g.Go(func() error {
			key := types.NamespacedName{
				Name:      statefulset.PodName(req.mariadb.ObjectMeta, i),
				Namespace: req.mariadb.Namespace,
			}
			var pod corev1.Pod
			if err := r.Get(ctx, key, &pod); err != nil {
				logger.V(1).Info("Error getting Pod when connecting replicas to new primary", "pod", key.Name)
				if apierrors.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("error getting pod: %w", err)
			}
			if !mariadbpod.PodReady(&pod) {
				logger.V(1).Info("Skipping non ready Pod when connecting replicas to new primary", "pod", key.Name)
				return nil
			}

			replClient, err := req.replClientSet.clientForIndex(ctx, i)
			if err != nil {
				return fmt.Errorf("error getting replica '%d' client: %v", i, err)
			}

			logger.V(1).Info("Connecting replica to new primary", "replica", i)

			if err := r.replConfigClient.ConfigureReplica(ctx, req.mariadb, replClient, newPrimary, replicaOpts...); err != nil {
				return fmt.Errorf("error configuring replica '%d': %v", i, err)
			}

			return nil
		})
	}

	return g.Wait()
}

func (r *ReplicationReconciler) changePrimaryToReplica(ctx context.Context, req *ReconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	if !req.currentPrimaryReady {
		logger.Info("Skipped changing primary to be a replica due to primary's non ready status")
		return nil
	}

	currentPrimary := *req.mariadb.Status.CurrentPrimaryPodIndex
	currentPrimaryClient, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}
	newPrimary := *ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{}).Primary.PodIndex
	newPrimaryClient, err := req.replClientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	logger.Info("Change primary to be a replica")
	r.recorder.Eventf(
		req.mariadb,
		nil,
		corev1.EventTypeNormal,
		mariadbv1alpha1.ReasonReplicationPrimaryToReplica,
		mariadbv1alpha1.ActionReconciling,
		"Unlocking primary '%d' and configuring it to be a replica. New primary at '%d'",
		currentPrimary,
		newPrimary,
	)

	replicaOpts, err := r.configureReplicaOpts(ctx, req, newPrimaryClient, logger)
	if err != nil {
		return fmt.Errorf("error getting replica options: %v", err)
	}
	// The old primary reattaches from its own position rather than from the new primary's. Once the replicas
	// were synced, every GTID the old primary holds is in the new primary's binary log, so its own position is
	// always servable, and any write the new primary accepted since its promotion (for example while a
	// stranded switchover was being resumed) is replayed on the old primary instead of being skipped.
	currentPrimaryPos, err := currentPrimaryClient.GtidCurrentPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary GTID position: %v", err)
	}
	if currentPrimaryPos != "" {
		replicaOpts = append(replicaOpts, WithGtidSlavePos(currentPrimaryPos))
	}

	logger.Info("Unlocking primary")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryLock,
		mariadbv1alpha1.ActionReconciling, "Unlocking primary")
	if err := currentPrimaryClient.UnlockTables(ctx); err != nil {
		return fmt.Errorf("error unlocking primary: %v", err)
	}

	logger.Info("Configuring primary to be a replica")
	return r.replConfigClient.ConfigureReplica(
		ctx,
		req.mariadb,
		currentPrimaryClient,
		newPrimary,
		replicaOpts...,
	)
}

func (r *ReplicationReconciler) configureReplicaOpts(ctx context.Context, req *ReconcileRequest, primaryClient *sql.Client,
	logger logr.Logger) ([]ConfigureReplicaOpt, error) {
	var replicaOpts []ConfigureReplicaOpt

	if req.replicasSynced {
		primaryBinlogPos, err := primaryClient.GtidBinlogPos(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting primary binlog position: %v", err)
		}
		logger.Info("Configuring replicas with primary GTID", "gtid", primaryBinlogPos)
		replicaOpts = append(replicaOpts, WithGtidSlavePos(primaryBinlogPos))
	} else {
		replicaOpts = append(replicaOpts, WithResetGtidSlavePos())
	}

	// avoid deleting binary logs during archival to prevent drifting from object storage
	if req.mariadb.IsPointInTimeRecoveryEnabled() {
		replicaOpts = append(replicaOpts, WithResetMaster(false))
	}
	return replicaOpts, nil
}

func (r *ReplicationReconciler) currentPrimaryReady(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *ReplicationClientSet) (bool, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return false, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	_, err := clientSet.clientForIndex(ctx, *mariadb.Status.CurrentPrimaryPodIndex, sql.WithTimeout(1*time.Second))
	return err == nil, nil
}
