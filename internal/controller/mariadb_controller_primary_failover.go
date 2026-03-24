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

	pvcUIDs, err := r.getStoragePVCUIDs(ctx, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting storage PVC UIDs: %v", err)
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

	if err := r.patch(ctx, mariadb, func(mdb *mariadbv1alpha1.MariaDB) error {
		replicationSpec := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
		replicationSpec.Enabled = true
		replicationSpec.Primary.PodIndex = candidateIndex
		mdb.Spec.Replication = &replicationSpec
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB primary: %v", err)
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.UpdateCurrentPrimary(mariadb, *candidateIndex)
		status.CurrentPrimaryFailingSince = nil
		condition.SetPrimarySwitched(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	logger.Info("Failing over primary after storage PVC change", "new-primary", candidateName, "new-primary-index", *candidateIndex)
	if r.Recorder != nil {
		r.Recorder.Eventf(
			mariadb,
			nil,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonPrimarySwitching,
			mariadbv1alpha1.ActionReconciling,
			"Failing over primary from index '%d' to index '%d' after storage PVC change",
			change.PodIndex,
			*candidateIndex,
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
