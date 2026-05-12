package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
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

	if primary == currentPrimary {
		return ctrl.Result{}, nil
	}

	if mdb.IsMultiClusterPrimary() {
		if err := r.resetPrimaryReplicaConnection(ctx, mdb, logger); err != nil {
			return ctrl.Result{}, fmt.Errorf("error resetting primary replica connection: %v", err)
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
			sql.WithConnectionName(replication.MultiClusterReplicaConnectionName),
		); err != nil && !sql.IsConnectionNotExists(err) {
			return fmt.Errorf("error stopping primary replica connection in Pod index %d: %v", i, err)
		}
		if err := client.ResetSlave(
			ctx,
			sql.WithConnectionName(replication.MultiClusterReplicaConnectionName),
		); err != nil && !sql.IsConnectionNotExists(err) {
			return fmt.Errorf("error resetting primary replica connection in Pod index %d: %v", i, err)
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
	return true
}
