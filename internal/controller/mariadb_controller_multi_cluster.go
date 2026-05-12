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
		if err := r.reconfigurePrimaryGtids(ctx, mdb, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling primary GTIDs: %v", err)
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

// reconfigurePrimaryGtids filters primary GTIDs based on the gtid_domain_id and sets up the replicas accordingly.
func (r *MariaDBReconciler) reconfigurePrimaryGtids(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) error {
	logger.Info("Reconfiguring primary GTIDs")

	clientSet := sql.NewClientSet(mdb, r.RefResolver)
	defer clientSet.Close()
	currentPrimaryPodIndex := *mdb.Status.CurrentPrimaryPodIndex

	client, err := sql.NewClientWithMariaDB(ctx, mdb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error getting SQL client set: %v", err)
	}
	defer client.Close()

	domainId, err := client.GtidDomainId(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_domain_id: %v", err)
	}

	binlogState, err := client.GtidBinlogState(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_binlog_state: %v", err)
	}
	if binlogState == "" {
		logger.Info("gtid_binlog_state is empty, skipping multi-cluster primary reconciliation...")
		return nil
	}
	allGtids, err := replication.ParseAllGtids(binlogState)
	if err != nil {
		return fmt.Errorf("error parsing gtid_binlog_state GTIDs %s: %v", binlogState, err)
	}
	primaryGtids := replication.FilterByDomain(allGtids, uint32(*domainId))

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
			if err := client.ResetBinlogState(ctx, replication.GtidsToString(primaryGtids)); err != nil {
				return fmt.Errorf("error resetting gtid_binlog_state in primary Pod: %v", err)
			}
			if err := client.ResetGtidSlavePos(ctx); err != nil && !sql.IsGtidSlavePosNoValueForDomain(err) {
				return fmt.Errorf("error resetting gtid_slave_pos in primary Pod: %v", err)
			}
		} else {
			if err := client.ResetBinlogState(ctx, replication.GtidsToString(primaryGtids)); err != nil {
				return fmt.Errorf("error resetting gtid_binlog_state in replica Pod index %d: %v", i, err)
			}
			if err := client.StopSlave(ctx); err != nil {
				return fmt.Errorf("error stopping replica: %v", err)
			}
			if err := client.StartSlave(ctx); err != nil {
				return fmt.Errorf("error starting replica: %v", err)
			}
		}
	}
	return nil
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
	if mdb.Status.CurrentPrimary == nil || mdb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("Current MariaDB primary not set, skipping multi-cluster reconciliation...")
		return false
	}
	return true
}
