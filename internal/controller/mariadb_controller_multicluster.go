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

	if err := client.ResetBinlogState(ctx, primaryGtids); err != nil {
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
	if err := client.ResetBinlogState(ctx, replicaGtid); err != nil {
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
	if !mdb.IsReplicationEnabled() {
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
	binlogPosGtids, err := replication.ParseAllGtids(rawBinlogPos)
	if err != nil {
		return fmt.Errorf("error parsing gtid_binlog_pos GTIDs: %v", err)
	}

	externalPrimaryClient, err := r.getExternalPrimaryClient(ctx, mdb)
	if err != nil {
		return fmt.Errorf("error getting external primary client: %v", err)
	}
	defer externalPrimaryClient.Close()

	externalDomainId, err := externalPrimaryClient.GtidDomainId(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_domain_id from external primary: %v", err)
	}
	if len(replication.FilterByDomain(binlogPosGtids, *externalDomainId)) > 0 {
		logger.Info(
			"External domain ID found in primary replica GTID, skipping reconciliation...",
			"domain-id", externalDomainId,
			"gitd", rawBinlogPos,
		)
		return nil
	}

	externalBinlogPos, err := externalPrimaryClient.GtidBinlogPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_pos from external primary: %v", err)
	}
	composedGtid, err := composeGtids(rawBinlogPos, externalBinlogPos)
	if err != nil {
		return fmt.Errorf("error composing GTIDs: %v", err)
	}

	if err := primaryClient.StopSlave(ctx, sql.WithConnectionName(replicationctrl.MultiClusterReplicaConnectionName)); err != nil {
		return fmt.Errorf("error stopping primary replica: %v", err)
	}
	if err := primaryClient.SetGtidSlavePos(ctx, composedGtid); err != nil {
		return fmt.Errorf("error setting gtid_slave_pos %s in primary replica: %v", composedGtid, err)
	}
	if err := primaryClient.StartSlave(ctx, sql.WithConnectionName(replicationctrl.MultiClusterReplicaConnectionName)); err != nil {
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

func composeGtids(rawGtid, rawExternalGtid string) (string, error) {
	gtids, err := parseGtids(rawGtid)
	if err != nil {
		return "", fmt.Errorf("error parsing GTID %s: %v", rawGtid, err)
	}
	externalGtids, err := parseGtids(rawExternalGtid)
	if err != nil {
		return "", fmt.Errorf("error parsing external GTID %s: %v", rawExternalGtid, err)
	}

	externalDomains := make(map[uint32]bool, len(externalGtids))
	merged := make([]replication.Gtid, 0, len(gtids)+len(externalGtids))
	for _, gtid := range externalGtids {
		externalDomains[gtid.DomainID] = true
		merged = append(merged, gtid)
	}
	for _, gtid := range gtids {
		if !externalDomains[gtid.DomainID] {
			merged = append(merged, gtid)
		}
	}
	return replication.GtidsToString(merged...), nil
}

func parseGtids(rawGtid string) ([]replication.Gtid, error) {
	if rawGtid == "" {
		return nil, nil
	}
	return replication.ParseAllGtids(rawGtid)
}
