package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	mxsstate "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/state"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// noPrimaryServerRecoveryDelay is how long the pool may report no primary server before the operator
	// restarts a MaxScale Pod. A failover settles in seconds, so a state that outlives this window is a
	// stale monitor topology rather than an operation in progress.
	noPrimaryServerRecoveryDelay = 5 * time.Minute
	// noPrimaryServerRecoveryRequeue paces Pod restarts so the monitor is given time to rediscover the
	// topology before the next Pod is considered.
	noPrimaryServerRecoveryRequeue = 30 * time.Second
)

func (r *MaxScaleReconciler) recordNoPrimaryServer(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) error {
	if mxs.Status.NoPrimaryServerSince != nil {
		return nil
	}
	r.Recorder.Eventf(
		mxs,
		nil,
		corev1.EventTypeWarning,
		mariadbv1alpha1.ReasonMaxScaleNoPrimaryServer,
		mariadbv1alpha1.ActionReconciling,
		"No server holds the Master state, the pool cannot route writes",
	)
	return r.patchStatus(ctx, mxs, func(mss *mariadbv1alpha1.MaxScaleStatus) error {
		mss.NoPrimaryServerSince = ptr.To(metav1.Now())
		return nil
	})
}

func (r *MaxScaleReconciler) clearNoPrimaryServer(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) error {
	if mxs.Status.NoPrimaryServerSince == nil {
		return nil
	}
	return r.patchStatus(ctx, mxs, func(mss *mariadbv1alpha1.MaxScaleStatus) error {
		mss.NoPrimaryServerSince = nil
		return nil
	})
}

// recoverStaleMonitorTopology restarts MaxScale Pods when the pool has been without a Master for longer
// than noPrimaryServerRecoveryDelay while MariaDB is observing a healthy primary. In that state the
// mariadbmon journal still names a node that is no longer the primary, and master_conditions blocks any
// promotion while that value stands. The journal is persisted on the MaxScale volume, so stopping and
// starting the monitor does not clear it: only a Pod restart forces a clean topology rediscovery.
func (r *MaxScaleReconciler) recoverStaleMonitorTopology(ctx context.Context, req *requestMaxScale,
	logger logr.Logger) (ctrl.Result, error) {
	since := req.mxs.Status.NoPrimaryServerSince
	if since == nil || time.Since(since.Time) < noPrimaryServerRecoveryDelay {
		return ctrl.Result{}, nil
	}
	if req.mxs.Spec.MariaDBRef == nil {
		return ctrl.Result{}, nil
	}
	mdb, err := r.getMariaDB(ctx, req)
	if err != nil {
		logger.V(1).Info("error getting MariaDB. Skipping stale monitor recovery...", "err", err)
		return ctrl.Result{}, nil
	}
	if !hasStaleMonitorTopology(req.mxs, mdb) {
		return ctrl.Result{}, nil
	}

	pod, err := r.staleMonitorPod(ctx, req.mxs, since.Time)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pod == nil {
		return ctrl.Result{}, nil
	}

	logger.Info(
		"Restarting MaxScale Pod to discard a stale monitor topology",
		"pod", pod.Name,
		"no-primary-since", since.Time,
	)
	if err := r.Delete(ctx, pod); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	r.Recorder.Eventf(
		req.mxs,
		nil,
		corev1.EventTypeWarning,
		mariadbv1alpha1.ReasonMaxScaleMonitorRestarted,
		mariadbv1alpha1.ActionReconciling,
		"Restarted Pod %s to discard a stale monitor topology",
		pod.Name,
	)
	return ctrl.Result{RequeueAfter: noPrimaryServerRecoveryRequeue}, nil
}

// hasStaleMonitorTopology reports whether MariaDB observes a primary that MaxScale is not able to see
// as Master. Restarting a Pod is only safe when a writable node actually exists: a pool without a Master
// during a switchover, or while no node is acting as primary, must be left alone.
func hasStaleMonitorTopology(mxs *mariadbv1alpha1.MaxScale, mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb == nil || !mdb.IsReplicationEnabled() || mdb.IsSwitchingPrimary() {
		return false
	}
	observedPrimary := mdb.ObservedPrimary()
	if observedPrimary == "" {
		return false
	}
	for _, srv := range mxs.Status.Servers {
		if srv.IsMaster() {
			return false
		}
	}
	for _, srv := range mxs.Status.Servers {
		if srv.Name == observedPrimary && mxsstate.IsRunning(srv.State) {
			return true
		}
	}
	return false
}

// staleMonitorPod returns the first MaxScale Pod that has been running since the pool lost its primary.
// A Pod created afterwards has already rediscovered the topology, so it is never restarted twice for
// the same incident.
func (r *MaxScaleReconciler) staleMonitorPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	since time.Time) (*corev1.Pod, error) {
	for i := 0; i < int(mxs.Spec.Replicas); i++ {
		key := types.NamespacedName{
			Name:      stsobj.PodName(mxs.ObjectMeta, i),
			Namespace: mxs.Namespace,
		}
		var pod corev1.Pod
		if err := r.Get(ctx, key, &pod); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if pod.DeletionTimestamp != nil || !pod.CreationTimestamp.Time.Before(since) {
			continue
		}
		return &pod, nil
	}
	return nil, nil
}
