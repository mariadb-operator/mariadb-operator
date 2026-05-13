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

	if !shouldReconcileMultiCluster(mdb, logger) {
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

	primaryClient, err := clientSet.ClientForIndex(ctx, currentPrimaryPodIndex)
	if err != nil {
		return fmt.Errorf("error getting primary client: %v", err)
	}

	domainId, err := primaryClient.GtidDomainId(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_domain_id: %v", err)
	}
	binlogState, err := primaryClient.GtidBinlogState(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_state: %v", err)
	}
	if binlogState == "" {
		logger.Info("gtid_binlog_state is empty, skipping reconciliation...")
		return nil
	}

	binlogStateGtids, err := replication.ParseAllGtids(binlogState)
	if err != nil {
		return fmt.Errorf("error parsing gtid_binlog_state GTIDs %s: %v", binlogState, err)
	}
	primaryGtids := replication.GtidsToString(
		replication.FilterByDomain(binlogStateGtids, uint32(*domainId))...,
	)

	podIndexes, err := mdb.OrderedPodIndexes()
	if err != nil {
		return fmt.Errorf("error getting ordered Pod indexes: %v", err)
	}

	for _, i := range podIndexes {
		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			return fmt.Errorf("error getting client for Pod index %d: %v", i, err)
		}

		if currentPrimaryPodIndex == i {
			if err := client.ResetBinlogState(ctx, primaryGtids); err != nil {
				return fmt.Errorf("error resetting gtid_binlog_state in primary Pod: %v", err)
			}
			// TODO: consider removing, as it always returns:
			// error reconciling MultiCluster: 1 error occurred:\n\t* error reconciling primary GTIDs:
			// error resetting gtid_slave_pos in primary Pod: Error 1948 (HY000):
			// Specified value for @@gtid_slave_pos contains no value for replication domain 1.
			// This conflicts with the binary log which contains GTID 1-20-4. If MASTER_GTID_POS=CURRENT_POS is used,
			// the binlog position will override the new value of @@gtid_slave_pos
			if err := client.ResetGtidSlavePos(ctx); err != nil && !sql.IsGtidSlavePosNoValueForDomain(err) {
				return fmt.Errorf("error resetting gtid_slave_pos in primary Pod: %v", err)
			}
		} else {
			if err := client.StopSlave(ctx); err != nil {
				return fmt.Errorf("error stopping replica: %v", err)
			}
			// TODO: replica may be lagged: filter current gtid_slave_pos by domain ID rather than pushing primary GTID
			if err := client.ResetBinlogState(ctx, primaryGtids); err != nil {
				return fmt.Errorf("error resetting gtid_binlog_state in replica Pod index %d: %v", i, err)
			}
			if err := client.SetGtidSlavePos(ctx, primaryGtids); err != nil {
				return fmt.Errorf("error setting gtid_slave_pos in replica Pod index %d: %v", i, err)
			}
			if err := client.StartSlave(ctx); err != nil {
				return fmt.Errorf("error starting replica: %v", err)
			}
		}
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

	binlogPos, err := primaryClient.GtidBinlogPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_pos: %v", err)
	}
	binlogPosGtids, err := replication.ParseAllGtids(binlogPos)
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
			"gitd", binlogPos,
		)
		return nil
	}

	externalBinlogPos, err := externalPrimaryClient.GtidBinlogPos(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_pos from external primary: %v", err)
	}
	composedGtid, err := composeGtids(binlogPos, externalBinlogPos)
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

func shouldReconcileMultiCluster(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) bool {
	if !mdb.IsMultiClusterEnabled() {
		return false
	}
	if mdb.HasPendingHATopologyConfiguration() ||
		mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() ||
		mdb.HasGaleraNotReadyCondition() ||
		mdb.IsInitializing() || mdb.IsScalingOut() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsUpdating() ||
		mdb.HasPendingBinlogReplay() {
		logger.V(1).Info("Ongoing MariaDB operation detected, skipping multi-cluster reconciliation...")
		return false
	}

	// TODO: avoid reconciling on MaxScale switchover?

	if mdb.Status.CurrentPrimary == nil || mdb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("Current MariaDB primary not set, skipping multi-cluster reconciliation...")
		return false
	}
	return true
}

func composeGtids(rawGtid, rawExternalGtid string) (string, error) {
	gtid, err := replication.ParseGtid(rawGtid)
	if err != nil {
		return "", fmt.Errorf("error parsing GTID %s: %v", rawGtid, err)
	}
	externalGtid, err := replication.ParseGtid(rawExternalGtid)
	if err != nil {
		return "", fmt.Errorf("error parsing external GTID %s: %v", rawExternalGtid, err)
	}
	return replication.GtidsToString(*gtid, *externalGtid), nil
}
