package replication

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	mdbpod "github.com/mariadb-operator/mariadb-operator/v26/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	mdbsts "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FailoverHandler struct {
	client      client.Client
	refResolver *refresolver.RefResolver
	mariadb     *mariadbv1alpha1.MariaDB
	logger      logr.Logger
}

func NewFailoverHandler(client client.Client, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) *FailoverHandler {
	return &FailoverHandler{
		client:      client,
		refResolver: refresolver.New(client),
		mariadb:     mariadb,
		logger:      logger,
	}
}

// FurthestAdvancedReplica finds a candidate to be promoted as primary, taking into account replica status.
func (f *FailoverHandler) FurthestAdvancedReplica(ctx context.Context) (string, error) {
	pods, err := mdbpod.ListMariaDBSecondaryPods(ctx, f.client, f.mariadb)
	if err != nil {
		return "", fmt.Errorf("error listing secondary Pods: %v", err)
	}
	f.logger.Info("Finding candidates to be promoted to primary")

	candidates := f.findCandidates(ctx, pods)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].name < candidates[j].name
	})
	if len(candidates) == 0 {
		return "", errors.New("no promotion candidates were found")
	}
	f.logger.Info("Found promotion candidates", "candidates", getCandidateNames(candidates))

	furthestAdvanced := f.furthestAdvancedCandidate(candidates)
	if furthestAdvanced == nil {
		return "", errors.New("no furthest advanced candidate was found")
	}
	return furthestAdvanced.name, nil
}

// PrimaryLossCandidate finds a candidate to be promoted after the primary lost its storage. Another failover
// authority, such as MaxScale auto_failover, may have promoted a replica before the operator noticed the loss.
// That replica is writable and no longer replicates, so FurthestAdvancedReplica skips it, and with no other
// replica left it finds no candidate at all. Being the node taking writes, it is the candidate, unless a replica
// still replicating is further advanced or more than one replica has been promoted.
func (f *FailoverHandler) PrimaryLossCandidate(ctx context.Context) (string, error) {
	pods, err := mdbpod.ListMariaDBSecondaryPods(ctx, f.client, f.mariadb)
	if err != nil {
		return "", fmt.Errorf("error listing secondary Pods: %v", err)
	}
	f.logger.Info("Finding candidates to be promoted to primary after primary storage loss")

	candidates, promoted := f.findPrimaryLossCandidates(ctx, pods)
	return f.primaryLossCandidate(promoted, candidates)
}

func (f *FailoverHandler) primaryLossCandidate(promoted, candidates []promotionCandidate) (string, error) {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].name < candidates[j].name
	})
	furthestAdvanced := f.furthestAdvancedCandidate(candidates)

	switch len(promoted) {
	case 0:
		if furthestAdvanced == nil {
			return "", errors.New("no promotion candidates were found")
		}
		return furthestAdvanced.name, nil
	case 1:
		if furthestAdvanced != nil {
			ahead, err := furthestAdvanced.gtidCurrentPos.GreaterThan(promoted[0].gtidCurrentPos)
			if err != nil {
				return "", fmt.Errorf("error comparing promoted replica %s with replica %s: %v",
					promoted[0].name, furthestAdvanced.name, err)
			}
			if ahead {
				return "", fmt.Errorf("promoted replica %s at %s is behind replica %s at %s",
					promoted[0].name, promoted[0].gtidCurrentPos, furthestAdvanced.name, furthestAdvanced.gtidCurrentPos)
			}
		}
		f.logger.Info("Found replica already promoted", "candidate", promoted[0].name)
		return promoted[0].name, nil
	default:
		return "", fmt.Errorf("more than one replica is writable: %v", getCandidateNames(promoted))
	}
}

func (f *FailoverHandler) findPrimaryLossCandidates(ctx context.Context,
	pods []corev1.Pod) (candidates []promotionCandidate, promoted []promotionCandidate) {
	for _, pod := range pods {
		podLogger := f.logger.WithValues("name", pod.Name)

		sqlClient, ok := f.candidateSQLClient(ctx, pod, podLogger)
		if !ok {
			continue
		}
		if candidate := f.promotedReplicaForClient(ctx, pod.Name, sqlClient, podLogger); candidate != nil {
			promoted = append(promoted, *candidate)
		} else if candidate := f.promotionCandidateForClient(ctx, pod.Name, sqlClient, podLogger); candidate != nil {
			candidates = append(candidates, *candidate)
		}
		sqlClient.Close()
	}
	return candidates, promoted
}

type promotedReplicaSQLClient interface {
	IsSystemVariableEnabled(ctx context.Context, variable string) (bool, error)
	IsReplicationReplica(ctx context.Context) (bool, error)
	GtidCurrentPos(ctx context.Context) (string, error)
	GtidDomainId(ctx context.Context) (*uint32, error)
}

func (f *FailoverHandler) promotedReplicaForClient(ctx context.Context, podName string,
	sqlClient promotedReplicaSQLClient, podLogger logr.Logger) *promotionCandidate {
	readOnly, err := sqlClient.IsSystemVariableEnabled(ctx, "read_only")
	if err != nil || readOnly {
		return nil
	}
	replica, err := sqlClient.IsReplicationReplica(ctx)
	if err != nil {
		podLogger.Info("Unable to determine replication state of writable Pod. Skipping...", "err", err)
		return nil
	}
	if replica {
		podLogger.Info("Writable Pod is still replicating. Skipping...")
		return nil
	}
	gtidDomainId, err := sqlClient.GtidDomainId(ctx)
	if err != nil {
		podLogger.Info("Error getting GTID domain ID of writable Pod. Skipping...", "err", err)
		return nil
	}
	rawGtid, err := sqlClient.GtidCurrentPos(ctx)
	if err != nil || rawGtid == "" {
		podLogger.Info("GTID current position of writable Pod not set. Skipping...", "err", err)
		return nil
	}
	gtidCurrentPos, err := replication.ParseFurthestGtidWithDomainId(rawGtid, *gtidDomainId, f.logger)
	if err != nil {
		podLogger.Info("Error parsing GTID current position of writable Pod. Skipping...", "err", err)
		return nil
	}
	return &promotionCandidate{
		name:           podName,
		gtidCurrentPos: gtidCurrentPos,
	}
}

func (f *FailoverHandler) candidateSQLClient(ctx context.Context, pod corev1.Pod, podLogger logr.Logger) (*sql.Client, bool) {
	if !mdbpod.PodReady(&pod) {
		podLogger.Info("Pod not ready. Skipping...")
		return nil, false
	}
	podIndex, err := mdbsts.PodIndex(pod.Name)
	if err != nil {
		podLogger.Info("Invalid Pod name. Skipping...", "err", err)
		return nil, false
	}
	if diverged, status := divergedReplica(f.mariadb, pod.Name); diverged {
		podLogger.Info("Replica diverged from the primary. Skipping...",
			"diverged-since", status.DivergedSince, "gtid-delta", ptr.Deref(status.GtidDelta, 0))
		return nil, false
	}
	sqlClient, err := sql.NewInternalClientWithPodIndex(ctx, f.mariadb, f.refResolver, *podIndex, sql.WithTimeout(3*time.Second))
	if err != nil {
		podLogger.Info("Unable to create SQL connection. Skipping...", "err", err)
		return nil, false
	}
	return sqlClient, true
}

// divergedReplica reports whether a replica stayed above the GTID delta threshold. Promoting it would discard
// every transaction it is missing: with a single replica the "furthest advanced" one is always that replica,
// however stale, so a diverged replica is never a promotion candidate.
func divergedReplica(mdb *mariadbv1alpha1.MariaDB, pod string) (bool, mariadbv1alpha1.ReplicaStatus) {
	replication := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	status, ok := replication.Replicas[pod]
	return ok && status.DivergedSince != nil, status
}

type promotionCandidate struct {
	name           string
	gtidCurrentPos *replication.Gtid
}

type promotionCandidateSQLClient interface {
	IsSystemVariableEnabled(ctx context.Context, variable string) (bool, error)
	ReplicaStatus(ctx context.Context, logger logr.Logger) (*mariadbv1alpha1.ReplicaStatusVars, error)
	GtidDomainId(ctx context.Context) (*uint32, error)
}

func (f *FailoverHandler) findCandidates(ctx context.Context, pods []corev1.Pod) []promotionCandidate {
	candidates := make([]promotionCandidate, 0, len(pods))
	for _, pod := range pods {
		podLogger := f.logger.WithValues("name", pod.Name)

		sqlClient, ok := f.candidateSQLClient(ctx, pod, podLogger)
		if !ok {
			continue
		}
		candidate := f.promotionCandidateForClient(ctx, pod.Name, sqlClient, podLogger)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
		sqlClient.Close()
	}
	return candidates
}

func (f *FailoverHandler) promotionCandidateForClient(ctx context.Context, podName string,
	sqlClient promotionCandidateSQLClient, podLogger logr.Logger) *promotionCandidate {
	readOnly, err := sqlClient.IsSystemVariableEnabled(ctx, "read_only")
	if err != nil {
		podLogger.Info("Unable to determine read_only state. Skipping...", "err", err)
		return nil
	}
	if !readOnly {
		podLogger.Info("Pod is writable. Skipping...")
		return nil
	}

	status, err := sqlClient.ReplicaStatus(ctx, podLogger)
	if err != nil {
		podLogger.Info("Unable to get replica status Skipping...", "err", err)
		return nil
	}

	// When the primary is down every replica reports a "Connecting" IO thread,
	// which must still qualify as a promotion candidate.
	if !status.IsIOThreadActive() {
		podLogger.Info("IO thread not running. Skipping...")
		return nil
	}
	if !status.IsSQLThreadRunning() {
		podLogger.Info("SQL thread not running. Skipping...")
		return nil
	}

	gtidDomainId, err := sqlClient.GtidDomainId(ctx)
	if err != nil {
		podLogger.Info("Error getting GTID domain ID. Skipping...", "err", err)
		return nil
	}

	hasRelayLogEvents, err := HasRelayLogEvents(status, *gtidDomainId, podLogger)
	if err != nil {
		podLogger.Info("Error checking relay log events. Skipping...", "err", err)
		return nil
	}
	if hasRelayLogEvents {
		podLogger.Info("Detected events in relay log. Skipping...")
		return nil
	}

	if status.GtidCurrentPos == nil {
		podLogger.Info("GTID current position not set. Skipping...")
		return nil
	}
	gtidCurrentPos, err := replication.ParseFurthestGtidWithDomainId(*status.GtidCurrentPos, *gtidDomainId, f.logger)
	if err != nil {
		podLogger.Info("Error parsing GTID current position. Skipping...", "err", err)
		return nil
	}

	return &promotionCandidate{
		name:           podName,
		gtidCurrentPos: gtidCurrentPos,
	}
}

func (f *FailoverHandler) furthestAdvancedCandidate(candidates []promotionCandidate) *promotionCandidate {
	var furthestAdvanced *promotionCandidate
	for i := range candidates {
		c := &candidates[i]
		candidateLogger := f.logger.WithValues("candidate", c.name)

		if c.gtidCurrentPos == nil {
			candidateLogger.Info("GTID position not set. Skipping...")
			continue
		}
		if furthestAdvanced == nil {
			furthestAdvanced = c
			continue
		}

		greaterThan, err := c.gtidCurrentPos.GreaterThan(furthestAdvanced.gtidCurrentPos)
		if err != nil {
			candidateLogger.Info("Error comparing GTID values. Skipping...", "err", err)
			continue
		}
		if greaterThan {
			furthestAdvanced = c
		}
	}
	return furthestAdvanced
}

func getCandidateNames(candidates []promotionCandidate) []string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.name
	}
	return names
}
