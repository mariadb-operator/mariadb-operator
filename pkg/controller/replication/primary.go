package replication

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	ds "github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	mxsclientset "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// currentPrimaryPodIndex returns the index of the current primary Pod.
//
// With MaxScale, 'status.currentPrimaryPodIndex' is copied from the cached MaxScale CR's 'status.primaryServer', which lags
// behind MaxScale while it switches over or fails over. Reconciling the replication with it may configure the Pod that
// MaxScale has just promoted as a replica of the old primary. Instead, the MaxScale API is queried live, via the MaxScale
// instance holding the cooperative monitoring lock, and the server in Master state is mapped to its Pod.
//
// It falls back to MariaDB's 'status.currentPrimaryPodIndex' when MaxScale is not enabled or not found, and when the MaxScale API
// cannot tell the primary: no instance holds the lock, the API is unreachable or no server is in Master state.
func (r *ReplicationReconciler) currentPrimaryPodIndex(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (*int, error) {
	statusPrimaryPodIndex := mariadb.Status.CurrentPrimaryPodIndex
	if !mariadb.IsMaxScaleEnabled() {
		return statusPrimaryPodIndex, nil
	}
	mxs, err := r.refResolver.MaxScale(ctx, mariadb.Spec.MaxScaleRef, mariadb.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return statusPrimaryPodIndex, nil
		}
		return nil, fmt.Errorf("error getting MaxScale: %v", err)
	}

	client, err := mxsclientset.NewClientSet(mxs, r.refResolver).PrimaryClient(ctx)
	if err != nil {
		logger.V(1).Info("Unable to get MaxScale primary client. Using MariaDB primary", "err", err)
		return statusPrimaryPodIndex, nil
	}
	if client == nil {
		logger.V(1).Info("MaxScale primary client not found. Using MariaDB primary")
		return statusPrimaryPodIndex, nil
	}
	serverIdx, err := client.Server.ListIndex(ctx)
	if err != nil {
		logger.V(1).Info("Unable to list MaxScale servers. Using MariaDB primary", "err", err)
		return statusPrimaryPodIndex, nil
	}

	for _, srv := range ds.Filter(serverIdx, mxs.ServerIDs()...) {
		if srv.Attributes == nil || !srv.Attributes.IsMaster() {
			continue
		}
		podIndex, err := mxs.ServerAddressPodIndex(mariadb, srv.Attributes.Parameters.Address)
		if err != nil {
			logger.V(1).Info("Unable to get MaxScale primary server Pod index. Using MariaDB primary", "server", srv.ID, "err", err)
			return statusPrimaryPodIndex, nil
		}
		if statusPrimaryPodIndex != nil && *statusPrimaryPodIndex != *podIndex {
			logger.Info("MariaDB's 'status.currentPrimaryPodIndex' is stale. Using MaxScale primary", "mariadb-primary", *statusPrimaryPodIndex,
				"maxscale-primary", *podIndex)
		}
		return podIndex, nil
	}
	logger.V(1).Info("MaxScale primary server not found. Using MariaDB primary")
	return statusPrimaryPodIndex, nil
}
