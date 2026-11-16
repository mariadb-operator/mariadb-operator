package controller

import (
	"context"
	"slices"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
)

// dataHoldingCurrentPrimary returns the current primary Pod while its storage PVC is the one it ran with. A replica
// recovery wipes the node it rebuilds, and the current primary is the node taking writes.
func dataHoldingCurrentPrimary(mariadb *mariadbv1alpha1.MariaDB, pvcUIDs map[int]string) string {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return ""
	}
	primary := *mariadb.Status.CurrentPrimaryPodIndex
	currentUID, ok := pvcUIDs[primary]
	if !ok || currentUID == "" {
		return ""
	}
	if storedUID, ok := storedStoragePVCUID(mariadb.Annotations, primary); ok && storedUID != "" && storedUID != currentUID {
		return ""
	}
	return stsobj.PodName(mariadb.ObjectMeta, primary)
}

// excludeCurrentPrimaryFromRecovery keeps the current primary out of a replica recovery. A recovery is decided for a
// replica, and the decision outlives a failover that promotes that replica: MaxScale auto_failover during a network
// partition promotes the replica whose replication stalled. Rebuilding it discards every write it took as primary and
// makes MaxScale fail back to the old primary. An episode recovering the current primary is ended.
func (r *MariaDBReconciler) excludeCurrentPrimaryFromRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	pvcUIDs map[int]string, activeRecoveryReplicas []string, logger logr.Logger) ([]string, string, error) {
	primary := dataHoldingCurrentPrimary(mariadb, pvcUIDs)
	if primary == "" || !slices.Contains(activeRecoveryReplicas, primary) {
		return activeRecoveryReplicas, primary, nil
	}
	logger.Info("Ending replica recovery of the current primary", "primary", primary)
	if r.Recorder != nil {
		r.Recorder.Eventf(mariadb,
			nil,
			corev1.EventTypeWarning,
			mariadbv1alpha1.ReasonMariaDBReplicaRecoveryRefused,
			mariadbv1alpha1.ActionReconciling,
			"Not recovering %s: it became the current primary", primary,
		)
	}
	if err := r.setReplicaToRecover(ctx, mariadb, ""); err != nil {
		return nil, "", err
	}
	return withoutReplica(activeRecoveryReplicas, primary), primary, nil
}

// recoveryReplicaSet merges the replicas to recover: the active episode, the immediate errors and the lost storage,
// and the error thresholds when recovery is enabled. The current primary is never part of it.
func recoveryReplicaSet(mariadb *mariadbv1alpha1.MariaDB, podStates map[int]podLifecycleState, replicaRecoveryEnabled bool,
	currentPrimary string, logger logr.Logger, replicaSets ...[]string) []string {
	if replicaRecoveryEnabled {
		replicaSets = append(replicaSets, getReplicasToRecover(mariadb, podStates, logger))
	}
	return withoutReplica(mergeReplicasToRecover(replicaSets...), currentPrimary)
}

func withoutReplica(replicas []string, replica string) []string {
	if replica == "" {
		return replicas
	}
	return slices.DeleteFunc(slices.Clone(replicas), func(name string) bool {
		return name == replica
	})
}
