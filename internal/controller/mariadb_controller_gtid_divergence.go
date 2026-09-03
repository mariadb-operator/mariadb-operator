package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	sqlclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// primaryGtidState is the reference GTID position of the current primary, used to measure how far
// behind each replica is. A replica whose relay log is empty because the primary rejects its GTID
// reports running threads and zero lag, so the GTID delta is the only signal that detects it.
type primaryGtidState struct {
	currentPos *string
	domainID   *uint32
}

func getPrimaryGtidState(ctx context.Context, clientSet primaryGtidClientSet,
	mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) primaryGtidState {
	if mdb.Status.CurrentPrimaryPodIndex == nil {
		return primaryGtidState{}
	}
	client, err := clientSet.ClientForIndex(ctx, *mdb.Status.CurrentPrimaryPodIndex)
	if err != nil {
		logger.Info("error getting client for primary Pod", "err", err)
		return primaryGtidState{}
	}

	currentPos, err := client.GtidCurrentPos(ctx)
	if err != nil {
		logger.Info("error getting primary GTID current position", "err", err)
		return primaryGtidState{}
	}
	domainID, err := client.GtidDomainId(ctx)
	if err != nil {
		logger.Info("error getting primary GTID domain ID", "err", err)
		return primaryGtidState{currentPos: ptr.To(currentPos)}
	}
	return primaryGtidState{
		currentPos: ptr.To(currentPos),
		domainID:   domainID,
	}
}

type primaryGtidClientSet interface {
	ClientForIndex(ctx context.Context, index int, opts ...sqlclient.Opt) (*sqlclient.Client, error)
}

// gtidDelta returns how many transactions the replica is behind the primary within the replication
// GTID domain. It returns nil when the delta cannot be determined, so the last observation is kept.
func gtidDelta(primary primaryGtidState, replicaPos *string, logger logr.Logger) *uint64 {
	if primary.currentPos == nil || primary.domainID == nil || replicaPos == nil {
		return nil
	}
	primaryGtid, err := replication.ParseFurthestGtidWithDomainId(*primary.currentPos, *primary.domainID, logger)
	if err != nil {
		logger.Info("error parsing primary GTID current position", "err", err)
		return nil
	}
	replicaGtid, err := replication.ParseFurthestGtidWithDomainId(*replicaPos, *primary.domainID, logger)
	if err != nil {
		logger.Info("error parsing replica GTID current position", "err", err)
		return nil
	}
	if replicaGtid.SequenceID >= primaryGtid.SequenceID {
		return ptr.To(uint64(0))
	}
	return ptr.To(primaryGtid.SequenceID - replicaGtid.SequenceID)
}

// setGtidDivergence records the observed GTID delta and the instant the replica first exceeded the
// divergence threshold. DivergedSince is held while the replica stays above the threshold and cleared
// as soon as it catches up, so a replica leaves maintenance on its own once it is usable again.
func setGtidDivergence(status *mariadbv1alpha1.ReplicaStatus, delta *uint64, maxDelta uint64, now time.Time) {
	if status == nil || delta == nil {
		return
	}
	status.GtidDelta = delta
	if maxDelta == 0 || *delta <= maxDelta {
		status.DivergedSince = nil
		return
	}
	if status.DivergedSince == nil {
		status.DivergedSince = ptr.To(metav1.NewTime(now))
	}
}

// recordGtidDivergenceTransition emits an event whenever a replica enters or leaves the diverged state,
// so that a stale replica is visible before it silently serves stale reads for an hour.
func (r *MariaDBReconciler) recordGtidDivergenceTransition(mdb *mariadbv1alpha1.MariaDB, pod string,
	previous, current *mariadbv1alpha1.ReplicaStatus) {
	previousDiverged := previous != nil && previous.DivergedSince != nil
	currentDiverged := current != nil && current.DivergedSince != nil

	if !previousDiverged && currentDiverged {
		r.Recorder.Eventf(
			mdb,
			nil,
			corev1.EventTypeWarning,
			mariadbv1alpha1.ReasonReplicationReplicaDiverged,
			mariadbv1alpha1.ActionReconciling,
			"Replica %s is %d transactions behind the primary, it will be taken out of the read pool in %s",
			pod,
			ptr.Deref(current.GtidDelta, 0),
			mdb.MaxGtidDeltaDuration(),
		)
		return
	}
	if previousDiverged && !currentDiverged {
		r.Recorder.Eventf(
			mdb,
			nil,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonReplicationReplicaCaughtUp,
			mariadbv1alpha1.ActionReconciling,
			"Replica %s has caught up with the primary",
			pod,
		)
	}
}
