package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	ctrl "sigs.k8s.io/controller-runtime"
)

const replicaRecoveryCheckedPrimaryAnnotationPrefix = "k8s.mariadb.com/replica-recovery-checked-primary-"

// replicaRecoveryCheckedPrimary is the primary the recovery checks of one episode ran against, together with the
// transactions the replica had received through replication at that time.
type replicaRecoveryCheckedPrimary struct {
	Episode         string `json:"episode"`
	PrimaryIndex    int    `json:"primaryIndex"`
	PrimaryPVCUID   string `json:"primaryPVCUID,omitempty"`
	ReplicaServerID uint32 `json:"replicaServerID,omitempty"`
	ReplicaGtids    string `json:"replicaGtids,omitempty"`
}

func replicaRecoveryCheckedPrimaryAnnotationKey(podIndex int) string {
	return fmt.Sprintf("%s%d", replicaRecoveryCheckedPrimaryAnnotationPrefix, podIndex)
}

func checkedRecoveryPrimary(mariadb *mariadbv1alpha1.MariaDB, podIndex int, recoveryID string) *replicaRecoveryCheckedPrimary {
	raw, ok := mariadb.Annotations[replicaRecoveryCheckedPrimaryAnnotationKey(podIndex)]
	if !ok {
		return nil
	}
	var checked replicaRecoveryCheckedPrimary
	if err := json.Unmarshal([]byte(raw), &checked); err != nil || checked.Episode != recoveryID {
		return nil
	}
	return &checked
}

// recheckRecoveryPrimary runs on every reconcile of a recovery episode whose checks already passed, until the
// replica data starts being replaced. The checks ran against the primary of that moment, and the backup the replica
// is rebuilt from is taken later: a primary that lost its storage, or a failover to a node behind the replica, in
// between would rebuild the replica from data that lacks its transactions. The primary storage is checked again, and
// a different primary than the checked one must hold the transactions the replica had received.
func (r *MariaDBReconciler) recheckRecoveryPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	replica string, podIndex int, recoveryID string, logger logr.Logger) (ctrl.Result, error) {
	current, result, err := r.verifyRecoveryPrimaryStorage(ctx, mariadb, replica, logger)
	if !result.IsZero() || err != nil {
		return result, err
	}
	checked := checkedRecoveryPrimary(mariadb, podIndex, recoveryID)
	if checked == nil ||
		(checked.PrimaryIndex == current.PrimaryIndex && checked.PrimaryPVCUID == current.PrimaryPVCUID) {
		return ctrl.Result{}, nil
	}

	logger.Info("Primary changed after the replica recovery checks, checking the new primary",
		"checked-primary", stsobj.PodName(mariadb.ObjectMeta, checked.PrimaryIndex), "checked-pvc-uid", checked.PrimaryPVCUID,
		"primary", stsobj.PodName(mariadb.ObjectMeta, current.PrimaryIndex), "pvc-uid", current.PrimaryPVCUID)
	gtids, err := r.readReplicaRecoveryGtids(ctx, mariadb, podIndex)
	if err != nil {
		reason := fmt.Sprintf("the primary changed to %s after the recovery checks and could not be checked: %v",
			stsobj.PodName(mariadb.ObjectMeta, current.PrimaryIndex), err)
		return r.refuseReplicaRecoveryFromPrimary(mariadb, replica, reason, logger), nil
	}
	if checked.ReplicaGtids != "" {
		if result := r.verifyPrimaryHoldsReplicatedTransactions(mariadb, replica, checked.ReplicaServerID,
			checked.ReplicaGtids, gtids, logger); !result.IsZero() {
			return result, nil
		}
	}

	current.Episode = recoveryID
	current.ReplicaServerID = checked.ReplicaServerID
	current.ReplicaGtids = checked.ReplicaGtids
	return ctrl.Result{}, r.recordReplicaRecoveryPreserved(ctx, mariadb, podIndex, recoveryID, current)
}
