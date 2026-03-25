package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	stspkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const primaryPVCFailoverRequeueDelay = 5 * time.Second

func shouldReconcilePrimaryPVCFailover(mdb *mariadbv1alpha1.MariaDB) bool {
	if !mdb.IsReplicationEnabled() || mdb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	if mdb.IsSwitchingPrimary() || mdb.IsScalingOut() || mdb.IsRecoveringReplicas() ||
		mdb.IsInitializing() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() {
		return false
	}
	return true
}

func (r *MariaDBReconciler) reconcilePrimaryPVCFailover(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcilePrimaryPVCFailover(mariadb) {
		return ctrl.Result{}, nil
	}

	pvcStates, err := r.getStoragePVCStates(ctx, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting storage PVC states: %v", err)
	}
	pvcUIDs := make(map[int]string, len(pvcStates))
	for i, state := range pvcStates {
		if state.UID != "" {
			pvcUIDs[i] = state.UID
		}
	}

	if candidateIndex := getInitialPrimaryPVCBootstrapCandidate(mariadb, pvcStates); candidateIndex != nil {
		currentPrimary := ptr.Deref(mariadb.Status.CurrentPrimaryPodIndex, 0)
		logger := log.FromContext(ctx).WithName("primary-failover").WithValues(
			"primary", stspkg.PodName(mariadb.ObjectMeta, currentPrimary),
			"pod-index", currentPrimary,
			"new-primary", stspkg.PodName(mariadb.ObjectMeta, *candidateIndex),
			"new-primary-index", *candidateIndex,
		)
		logger.Info("Fresh primary storage detected during bootstrap, promoting reusable replica PVC as primary")
		return r.switchPrimaryToIndex(ctx, mariadb, currentPrimary, *candidateIndex, logger)
	}

	change, ok := getPrimaryPVCChange(mariadb, pvcUIDs)
	if !ok {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx).WithName("primary-failover").WithValues(
		"primary", stspkg.PodName(mariadb.ObjectMeta, change.PodIndex),
		"pod-index", change.PodIndex,
		"previous-uid", change.StoredUID,
		"current-uid", change.CurrentUID,
	)
	logger.Info("Primary storage PVC changed, selecting failover candidate")

	candidateName, err := r.selectFailoverCandidate(ctx, mariadb, logger)
	if err != nil {
		if r.Recorder != nil {
			r.Recorder.Eventf(
				mariadb,
				nil,
				corev1.EventTypeWarning,
				mariadbv1alpha1.ReasonPrimarySwitching,
				mariadbv1alpha1.ActionReconciling,
				"Primary storage PVC changed for index '%d', but no failover candidate is ready: %v",
				change.PodIndex,
				err,
			)
		}
		logger.Info("Primary storage PVC changed, but no failover candidate is ready. Requeuing...", "err", err)
		return ctrl.Result{RequeueAfter: primaryPVCFailoverRequeueDelay}, nil
	}

	candidateIndex, err := stspkg.PodIndex(candidateName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting failover candidate Pod index: %v", err)
	}

	return r.switchPrimaryToIndex(ctx, mariadb, change.PodIndex, *candidateIndex, logger)
}

func (r *MariaDBReconciler) switchPrimaryToIndex(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	currentPrimaryIndex int, candidateIndex int, logger logr.Logger) (ctrl.Result, error) {
	if err := r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) error {
		replicationSpec := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
		replicationSpec.Enabled = true
		replicationSpec.Primary.PodIndex = ptr.To(candidateIndex)
		mdb.Spec.Replication = &replicationSpec
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB primary: %v", err)
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.UpdateCurrentPrimary(mariadb, candidateIndex)
		status.CurrentPrimaryFailingSince = nil
		condition.SetPrimarySwitched(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	candidateName := stspkg.PodName(mariadb.ObjectMeta, candidateIndex)
	logger.Info("Switching primary", "new-primary", candidateName, "new-primary-index", candidateIndex)
	if r.Recorder != nil {
		r.Recorder.Eventf(
			mariadb,
			nil,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonPrimarySwitching,
			mariadbv1alpha1.ActionReconciling,
			"Switching primary from index '%d' to index '%d'",
			currentPrimaryIndex,
			candidateIndex,
		)
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *MariaDBReconciler) selectFailoverCandidate(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (string, error) {
	if r.FailoverCandidateFn != nil {
		return r.FailoverCandidateFn(ctx, mariadb, logger)
	}
	return replication.NewFailoverHandler(r.Client, mariadb, logger.V(1)).FurthestAdvancedReplica(ctx)
}
