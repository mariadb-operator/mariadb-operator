package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	replicationctrl "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileMultiCluster(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("multi-cluster")

	shouldReconcile, err := r.shouldReconcileMultiCluster(ctx, mdb, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error determining whether multi-cluster should be reconciled: %v", err)
	}
	if !shouldReconcile {
		return ctrl.Result{}, nil
	}
	multiCluster := ptr.Deref(mdb.Spec.MultiCluster, mariadbv1alpha1.MultiCluster{})
	primary := multiCluster.Primary
	currentPrimary := ptr.Deref(mdb.Status.CurrentMultiClusterPrimary, "")

	// during provisioning, cluster-level switchover reconciliation is not performed
	if currentPrimary == "" {
		return ctrl.Result{}, r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			status.CurrentMultiClusterPrimary = &primary
			return nil
		})
	}
	if primary == currentPrimary {
		return ctrl.Result{}, nil
	}

	if mdb.IsMultiClusterPrimary() {
		if err := r.resetPrimaryReplicaConnection(ctx, mdb, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error resetting primary replica connection: %v", err)
		}
		if err := r.reconfigurePrimaryClusterGtids(ctx, mdb, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling primary cluster GTIDs: %v", err)
		}
	} else {
		if err := r.reconfigureReplicaClusterGtids(ctx, mdb, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling replica cluster GTIDs: %v", err)
		}
	}

	return ctrl.Result{}, r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.CurrentMultiClusterPrimary = &primary
		return nil
	})
}

func (r *MariaDBReconciler) resetPrimaryReplicaConnection(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) error {
	logger.Info("Resetting primary replica connection")

	clientSet := sql.NewClientSet(mdb, r.RefResolver)
	defer clientSet.Close()

	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			return fmt.Errorf("error getting client for Pod index %d: %v", i, err)
		}
		if err := client.StopSlave(
			ctx,
			sql.WithConnectionName(replicationctrl.MultiClusterReplicaConnectionName),
		); err != nil && !sql.IsConnectionNotExists(err) {
			return fmt.Errorf("error stopping primary replica connection in Pod index %d: %v", i, err)
		}
		if err := client.ResetSlave(
			ctx,
			sql.WithConnectionName(replicationctrl.MultiClusterReplicaConnectionName),
		); err != nil && !sql.IsConnectionNotExists(err) {
			return fmt.Errorf("error resetting primary replica connection in Pod index %d: %v", i, err)
		}
	}
	return nil
}

// reconfigurePrimaryClusterGtids filters primary GTIDs based on its gtid_domain_id and sets up the replicas accordingly.
func (r *MariaDBReconciler) reconfigurePrimaryClusterGtids(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) error {
	if !mdb.IsReplicationEnabled() {
		return nil
	}
	logger.Info("Reconfiguring primary GTIDs")

	clientSet := sql.NewClientSet(mdb, r.RefResolver)
	defer clientSet.Close()
	currentPrimaryPodIndex := *mdb.Status.CurrentPrimaryPodIndex

	podIndexes, err := mdb.OrderedPodIndexes()
	if err != nil {
		return fmt.Errorf("error getting ordered Pod indexes: %v", err)
	}

	for _, i := range podIndexes {
		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			return fmt.Errorf("error getting client for Pod index %d: %v", i, err)
		}
		domainId, err := client.GtidDomainId(ctx)
		if err != nil {
			return fmt.Errorf("error getting gtid_domain_id for Pod index %d: %v", i, err)
		}

		if currentPrimaryPodIndex == i {
			if err := r.filterPrimaryBinlogByDomain(ctx, mdb, uint32(*domainId), client, logger); err != nil {
				return fmt.Errorf("error filtering primary gtid_binlog_state by domain: %v", err)
			}
		} else {
			if err := r.filterReplicaGtidByDomain(ctx, uint32(*domainId), i, client); err != nil {
				return fmt.Errorf("error filtering replica gtid_slave_pos by domain: %v", err)
			}
		}
	}
	return nil
}

func (r *MariaDBReconciler) filterPrimaryBinlogByDomain(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, domainId uint32,
	client *sql.Client, logger logr.Logger) error {
	rawBinlogState, err := client.GtidBinlogState(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_state: %v", err)
	}
	if rawBinlogState == "" {
		logger.Info("gtid_binlog_state is empty, skipping reconciliation...")
		return nil
	}
	binlogStateGtids, err := replication.ParseAllGtids(rawBinlogState)
	if err != nil {
		return fmt.Errorf("error parsing gtid_binlog_state GTIDs %s: %v", rawBinlogState, err)
	}
	primaryGtids := replication.GtidsToString(
		replication.FilterByDomain(binlogStateGtids, domainId)...,
	)

	if err := client.SetBinlogState(ctx, primaryGtids); err != nil {
		return fmt.Errorf("error resetting gtid_binlog_state in primary Pod: %v", err)
	}
	if err := replicationctrl.PauseGtidStrictMode(ctx, mdb, client, r.Client, logger.V(1)); err != nil {
		return fmt.Errorf("error pausing gtid_strict_mode in primary Pod: %v", err)
	}
	if err := client.ResetGtidSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting gtid_slave_pos in primary Pod: %v", err)
	}
	if err := replicationctrl.ResumeGtidStrictMode(ctx, mdb, client, r.Client, logger.V(1)); err != nil {
		return fmt.Errorf("error resuming gtid_strict_mode in primary Pod: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) filterReplicaGtidByDomain(ctx context.Context, domainId uint32, podIndex int,
	client *sql.Client) error {
	rawReplicaGtid, err := client.GtidCurrentPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting replica GTID: %v", err)
	}
	replicaGtids, err := replication.ParseAllGtids(rawReplicaGtid)
	if err != nil {
		return fmt.Errorf("error parsing replica GTID: %v", err)
	}
	replicaGtid := replication.GtidsToString(
		replication.FilterByDomain(replicaGtids, domainId)...,
	)

	if err := client.StopSlave(ctx); err != nil {
		return fmt.Errorf("error stopping replica in replica Pod index %d: %v", podIndex, err)
	}
	if err := client.SetBinlogState(ctx, replicaGtid); err != nil {
		return fmt.Errorf("error resetting gtid_binlog_state in replica Pod index %d: %v", podIndex, err)
	}
	if err := client.SetGtidSlavePos(ctx, replicaGtid); err != nil {
		return fmt.Errorf("error setting gtid_slave_pos in replica Pod index %d: %v", podIndex, err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return fmt.Errorf("error starting replica in replica Pod index %d: %v", podIndex, err)
	}
	return nil
}

// reconfigureReplicaClusterGtids sets up primary replica based on its own gtid_binlog_pos and the one from the primary cluster.
func (r *MariaDBReconciler) reconfigureReplicaClusterGtids(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	if !mdb.IsReplicationEnabled() && !mdb.IsGaleraEnabled() {
		return nil
	}
	logger.Info("Reconfiguring replica GTIDs")

	primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, mdb, r.RefResolver, *mdb.Status.CurrentPrimaryPodIndex)
	if err != nil {
		return fmt.Errorf("error getting primary client: %v", err)
	}
	defer primaryClient.Close()

	rawBinlogPos, err := primaryClient.GtidBinlogPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_pos: %v", err)
	}
	if rawBinlogPos == "" {
		logger.Info("gtid_binlog_pos is empty, skipping reconciliation...")
		return nil
	}
	binlogPosGtids, err := replication.ParseAllGtids(rawBinlogPos)
	if err != nil {
		return fmt.Errorf("error parsing gtid_binlog_pos GTIDs: %v", err)
	}

	externalPrimaryClient, err := r.getExternalPrimaryClient(ctx, mdb)
	if err != nil {
		return fmt.Errorf("error getting external primary client: %v", err)
	}
	defer externalPrimaryClient.Close()

	externalBinlogPos, err := externalPrimaryClient.GtidBinlogPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_pos from external primary: %v", err)
	}
	if externalBinlogPos == "" {
		logger.Info("gtid_binlog_pos in external primary is empty, skipping reconciliation...")
		return nil
	}
	externalGtids, err := replication.ParseAllGtids(externalBinlogPos)
	if err != nil {
		return fmt.Errorf("error parsing external gtid_binlog_pos GTIDs: %v", err)
	}

	// The domains of the primary cluster are only present in the primary replica once it has replicated from it.
	// Unlike the replication topology, a Galera primary cluster keeps the domains of the replica clusters in its binary
	// logs, hence all of them must be checked.
	externalDomains := make([]uint32, len(externalGtids))
	for i, gtid := range externalGtids {
		externalDomains[i] = gtid.DomainID
	}
	if len(replication.FilterByDomain(binlogPosGtids, externalDomains...)) == len(externalDomains) {
		logger.Info(
			"External domains found in primary replica GTID, skipping reconciliation...",
			"domains", externalDomains,
			"gtid", rawBinlogPos,
		)
		return nil
	}

	composedGtid, err := composeGtids(rawBinlogPos, externalBinlogPos)
	if err != nil {
		return fmt.Errorf("error composing GTIDs: %v", err)
	}

	// The multi-cluster connection doesn't exist yet in a Galera cluster being demoted: it is configured by the Galera
	// controller in the next reconciliation cycle, once the cluster switchover has been reconciled.
	if err := primaryClient.StopSlave(
		ctx,
		sql.WithConnectionName(replicationctrl.MultiClusterReplicaConnectionName),
	); err != nil && !sql.IsConnectionNotExists(err) {
		return fmt.Errorf("error stopping primary replica: %v", err)
	}
	if err := primaryClient.SetGtidSlavePos(ctx, composedGtid); err != nil {
		return fmt.Errorf("error setting gtid_slave_pos %s in primary replica: %v", composedGtid, err)
	}
	if err := primaryClient.StartSlave(
		ctx,
		sql.WithConnectionName(replicationctrl.MultiClusterReplicaConnectionName),
	); err != nil && !sql.IsConnectionNotExists(err) {
		return fmt.Errorf("error starting primary replica: %v", err)
	}
	return nil
}

func (r *MariaDBReconciler) getExternalPrimaryClient(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*sql.Client, error) {
	externalMariaDBRef, err := mdb.Spec.MultiCluster.GetExternalMariaDBRefForMember(mdb.Spec.MultiCluster.Primary)
	if err != nil {
		return nil, fmt.Errorf("error finding externalMariaDBRef for primary member: %v", err)
	}
	externalMariaDB, err := r.RefResolver.ExternalMariaDB(ctx, externalMariaDBRef, mdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting primary ExternalMariaDB: %v", err)
	}
	externalPrimaryClient, err := sql.NewClientWithMariaDB(ctx, externalMariaDB, r.RefResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating external primary client: %v", err)
	}
	return externalPrimaryClient, nil
}

func (r *MariaDBReconciler) shouldReconcileMultiCluster(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (bool, error) {
	if !mdb.IsMultiClusterEnabled() {
		return false, nil
	}
	if mdb.Status.CurrentPrimary == nil || mdb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("Current MariaDB primary not set, skipping multi-cluster reconciliation...")
		return false, nil
	}
	if mdb.HasPendingHATopologyConfiguration() ||
		mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() ||
		mdb.HasGaleraNotReadyCondition() ||
		mdb.IsInitializing() || mdb.IsScalingOut() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsUpdating() ||
		mdb.HasPendingBinlogReplay() {
		logger.V(1).Info("Ongoing MariaDB operation detected, skipping multi-cluster reconciliation...")
		return false, nil
	}
	if mdb.IsMaxScaleEnabled() {
		mxs, err := r.RefResolver.MaxScale(ctx, mdb.Spec.MaxScaleRef, mdb.Namespace)
		if err != nil {
			return false, fmt.Errorf("error getting MaxScale: %v", err)
		}
		if mxs.IsSwitchingPrimary() {
			logger.V(1).Info("Ongoing MaxScale switchover detected, skipping multi-cluster reconciliation...")
			return false, nil
		}
	}
	return true, nil
}

// composeGtids merges the GTIDs of a replica cluster with the ones of its primary cluster by replication domain.
// The replica cluster GTIDs (local GTIDs) take precedence, for the following reasons:
// - Multi-cluster switchover assumes the replica is synced at the time of doing the operation, maintenance mode is provided for achieving this.
// - Replica cluster should never have more recent GTIDs than the primary cluster, writes are not allowed.
// - Multi-cluster CHANGE MASTER statement uses gtid_slave_pos as initial offset.
// In the following scenario:
// - Replica cluster: 0-1-4
// - Primary cluster: 0-1-7,0-10-3
// The resulting GTID will be: 0-1-4,0-10-3. This preserves the replica position.
func composeGtids(rawGtid, rawExternalGtid string) (string, error) {
	externalGtids, err := replication.ParseAllGtids(rawExternalGtid)
	if err != nil {
		return "", fmt.Errorf("error parsing external GTID %s: %v", rawExternalGtid, err)
	}
	gtids, err := replication.ParseAllGtids(rawGtid)
	if err != nil {
		return "", fmt.Errorf("error parsing GTID %s: %v", rawGtid, err)
	}
	return replication.GtidsToString(replication.MergeByDomain(externalGtids, gtids)...), nil
}
