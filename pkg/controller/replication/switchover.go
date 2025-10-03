package replication

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

type switchoverPhase struct {
	name      string
	reconcile func(context.Context, *reconcileRequest, logr.Logger) error
}

func isSwitchoverStale(mdb *mariadbv1alpha1.MariaDB) bool {
	return mdb.IsSwitchingPrimary() && !mdb.IsSwitchoverRequired()
}

func shouldReconcileSwitchover(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsMaxScaleEnabled() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() {
		return false
	}
	if !mdb.HasConfiguredReplica() {
		return false
	}
	return mdb.IsSwitchoverRequired()
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *reconcileRequest, switchoverLogger logr.Logger) error {
	logger := switchoverLogger.WithValues("mariadb", req.mariadb.Name)

	if err := r.reconcileStaleSwitchover(ctx, req, logger); err != nil {
		return fmt.Errorf("error reconciling stale switchover: %v", err)
	}
	if !shouldReconcileSwitchover(req.mariadb) {
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
			name:      "Wait for replica sync",
			reconcile: r.waitForReplicaSync,
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
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.UpdateCurrentPrimary(req.mariadb, newPrimary)
		condition.SetPrimarySwitched(&req.mariadb.Status)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}

	logger.Info("Primary switched")
	r.recorder.Eventf(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitched,
		"Primary switched from index '%d' to index '%d'", *primary, newPrimary)
	return nil
}

func (r *ReplicationReconciler) reconcileStaleSwitchover(ctx context.Context, req *reconcileRequest,
	logger logr.Logger) error {
	if !isSwitchoverStale(req.mariadb) {
		return nil
	}
	ready, err := r.currentPrimaryReady(ctx, req.mariadb, req.replClientSet)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		logger.Info("Skipped stale switchover reconciliation due to primary's non ready status")
		return nil
	}
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

	logger.Info("Stale switchover has been reset")
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationResetStaleSwitchover,
		"Stale switchover has been reset")
	return nil
}

func (r *ReplicationReconciler) lockPrimaryWithReadLock(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	ready, err := r.currentPrimaryReady(ctx, req.mariadb, req.replClientSet)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		logger.Info("Skipped locking primary with read lock due to primary's non ready status")
		return nil
	}
	client, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	logger.Info("Locking primary with read lock")
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryLock,
		"Locking primary with read lock")
	return client.LockTablesWithReadLock(ctx)
}

func (r *ReplicationReconciler) setPrimaryReadOnly(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	ready, err := r.currentPrimaryReady(ctx, req.mariadb, req.replClientSet)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		logger.Info("Skipped enabling readonly mode in primary due to primary's non ready status")
		return nil
	}
	client, err := req.replClientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	logger.Info("Enabling readonly mode in primary")
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryReadonly,
		"Enabling readonly mode in primary")
	return client.EnableReadOnly(ctx)
}

func (r *ReplicationReconciler) waitForReplicaSync(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	ready, err := r.currentPrimaryReady(ctx, req.mariadb, req.replClientSet)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
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
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationReplicaSync,
		"Waiting for replicas to be synced with primary")
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
			timeout := replication.Replica.SyncTimeout.Duration
			if err := replClient.WaitForReplicaGtid(ctx, primaryGtid, timeout); err != nil {
				logger.Error(err, "Error waiting for GTID in replica", "gtid", primaryGtid, "replica", i)
				r.recorder.Eventf(req.mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationReplicaSyncErr,
					"Error waiting for GTID '%s' in replica '%d': %v", primaryGtid, i, err)

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

func (r *ReplicationReconciler) configureNewPrimary(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	newPrimary := *ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{}).Primary.PodIndex
	newPrimaryClient, err := req.replClientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	logger.Info("Configuring new primary")
	r.recorder.Eventf(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryNew,
		"Configuring new primary at index '%d'", newPrimary)

	if err := r.replConfigClient.ConfigurePrimary(ctx, req.mariadb, newPrimaryClient); err != nil {
		return fmt.Errorf("error confguring new primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) connectReplicasToNewPrimary(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}

	newPrimary := *ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{}).Primary.PodIndex
	newPrimaryClient, err := req.replClientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	logger.Info("Connecting replicas to new primary")
	r.recorder.Eventf(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationReplicaConn,
		"Connecting replicas to new primary at '%d'", newPrimary)

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

func (r *ReplicationReconciler) changePrimaryToReplica(ctx context.Context, req *reconcileRequest, logger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	ready, err := r.currentPrimaryReady(ctx, req.mariadb, req.replClientSet)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
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
	r.recorder.Eventf(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryToReplica,
		"Unlocking primary '%d' and configuring it to be a replica. New primary at '%d'", currentPrimary, newPrimary)

	replicaOpts, err := r.configureReplicaOpts(ctx, req, newPrimaryClient, logger)
	if err != nil {
		return fmt.Errorf("error getting replica options: %v", err)
	}

	logger.Info("Unlocking primary")
	r.recorder.Event(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryLock, "Unlocking primary")
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

func (r *ReplicationReconciler) configureReplicaOpts(ctx context.Context, req *reconcileRequest, primaryClient *sql.Client,
	logger logr.Logger) ([]ConfigureReplicaOpt, error) {
	if req.replicasSynced {
		primaryBinlogPos, err := primaryClient.GtidBinlogPos(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting primary binlog position: %v", err)
		}
		logger.Info("Configuring replicas with primary GTID", "gtid", primaryBinlogPos)
		return []ConfigureReplicaOpt{
			WithGtidSlavePos(primaryBinlogPos),
		}, nil
	}
	return []ConfigureReplicaOpt{
		WithResetGtidSlavePos(),
	}, nil
}

func (r *ReplicationReconciler) currentPrimaryReady(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *ReplicationClientSet) (bool, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return false, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	_, err := clientSet.clientForIndex(ctx, *mariadb.Status.CurrentPrimaryPodIndex, sql.WithTimeout(1*time.Second))
	return err == nil, nil
}
