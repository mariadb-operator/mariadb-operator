package replication

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

// TestConfigureReplicaOpts_HardFailover_ResetsGtid documents the bug:
//
//	During a hard failover (primary node unreachable), the switchover reconciler calls
//	waitForNewPrimarySync instead of waitForReplicaSync. Unlike waitForReplicaSync,
//	waitForNewPrimarySync does NOT set req.replicasSynced = true. As a result,
//	configureReplicaOpts falls through to the replicasSynced=false branch and returns
//	WithResetGtidSlavePos(), which calls SET @@global.gtid_slave_pos='' on every
//	surviving replica. When the replicas reconnect to the new primary, they request the
//	full binlog history from position 0 and re-apply transactions they already have,
//	causing ER_DUP_ENTRY (errno 1062) and a stuck replication thread.
//
// This test currently FAILS (asserts the desired correct behavior, which the bug violates).
// It will PASS once the fix is applied (e.g. setting req.replicasSynced = true inside
// waitForNewPrimarySync after it confirms the new primary has no more relay-log events).
func TestConfigureReplicaOpts_HardFailover_ResetsGtid(t *testing.T) {
	r := &ReplicationReconciler{}

	// Simulate the state produced by a hard failover: the old primary is dead so
	// waitForNewPrimarySync is called (not waitForReplicaSync), and replicasSynced
	// is never set to true.
	req := &ReconcileRequest{
		mariadb:             &mariadbv1alpha1.MariaDB{}, // non-nil; no PITR or replication configured
		currentPrimaryReady: false,                      // old primary is unreachable
		replicasSynced:      false,                      // waitForNewPrimarySync never sets this
	}

	// primaryClient is nil: the replicasSynced=false path never dereferences it.
	opts, err := r.configureReplicaOpts(context.Background(), req, nil, logr.Discard())
	if err != nil {
		t.Fatalf("configureReplicaOpts returned unexpected error: %v", err)
	}

	// Post-fix: returning zero opts is correct for hard failover with PITR disabled.
	// It means gtid_slave_pos is left untouched on the replica, which is the right behavior.
	cro := ConfigureReplicaOpts{}
	for _, opt := range opts {
		opt(&cro)
	}

	// Replicas must NOT have their GTID position wiped. Resetting gtid_slave_pos to ''
	// would cause them to replay all binlog events from position 0 on the new primary,
	// hitting ER_DUP_ENTRY (errno 1062) on rows they have already applied.
	if cro.ResetGtidSlavePos {
		t.Errorf(
			"configureReplicaOpts must not set ResetGtidSlavePos=true in the hard failover path " +
				"(currentPrimaryReady=false). Doing so wipes the GTID slave position on surviving " +
				"replicas, causing them to replay already-applied transactions and hit ER_DUP_ENTRY " +
				"(errno 1062). Surviving replicas should reconnect from their last applied GTID.",
		)
	}
}

// TestWaitForReplicaSync_ReconnectsDisconnectedReplica documents the bug:
//
//	During a planned switchover, the phases are:
//	  1. Lock primary (FLUSH TABLES WITH READ LOCK)
//	  2. Set read_only on primary
//	  3. Wait for replica sync (MASTER_GTID_WAIT)
//	  4. Configure new primary (STOP SLAVE / RESET SLAVE ALL on the new primary)
//	  5. Connect remaining replicas to new primary
//	  6. Change old primary to replica
//
//	If a previous iteration completed phase 4 but failed in phase 5 or 6, the new
//	primary has no slave configured.  The next reconcile loop retries from phase 1,
//	but phase 3 calls MASTER_GTID_WAIT on a replica that can never advance its GTID
//	(no slave running) – timing out on every attempt and leaving the cluster stuck
//	with the old primary in read_only=ON indefinitely.
//
//	The fix: before issuing MASTER_GTID_WAIT, waitForReplicaSync checks whether the
//	replica has a slave configured (IsReplicationReplica).  If not, it calls
//	ConfigureReplica with WithResetMaster(false) to reconnect the replica to the
//	current primary so it can catch up, then issues the GTID wait.
//
//	Because this path requires live SQL connections it is covered by integration
//	tests.  This comment serves as the design record.
func TestWaitForReplicaSync_ReconnectsDisconnectedReplica(_ *testing.T) {
	// Integration-test only; documented here for design record.
	// See pkg/controller/replication/switchover.go: waitForReplicaSync.
}

// TestConfigureReplicaOpts_PlannedSwitchover_DoesNotResetGtid verifies that the planned
// switchover path (old primary still reachable, replicas fully synced) does NOT reset GTID.
// This path was already correct; the test serves as a regression guard.
//
// Because replicasSynced=true requires a live SQL call to the primary
// (primaryClient.GtidBinlogPos), this case is covered by the integration test suite.
// We document it here for completeness.
func TestConfigureReplicaOpts_PlannedSwitchover_ReplicasSyncedTrue_DoesNotReset(t *testing.T) {
	// When replicasSynced=true, configureReplicaOpts calls primaryClient.GtidBinlogPos,
	// which requires a real SQL connection and therefore cannot be unit-tested without
	// a mock. This test serves only as documentation; the integration test covers it.
	t.Skip("planned-switchover path requires a live SQL connection to the primary; covered by integration tests")
}

// TestWaitForNewPrimarySync_NoSlaveSkipsRelayLogWait documents the bug:
//
//	When the current primary is unreachable (currentPrimaryReady=false), waitSync
//	dispatches to waitForNewPrimarySync instead of waitForReplicaSync.  That path
//	polls the new primary's SHOW REPLICA STATUS to confirm its relay log has been
//	drained before promotion.
//
//	If a previous switchover iteration already completed phase 4 (configureNewPrimary
//	calls STOP ALL SLAVES / RESET SLAVE ALL on the new primary), the slave entry on
//	the new primary is gone.  SHOW REPLICA STATUS returns no rows, so ReplicaStatus
//	produces a struct with Gtid_IO_Pos == nil, and HasRelayLogEvents errors with
//	"GTID IO position must be set".  PollUntilSuccessOrContextCancel keeps retrying
//	until the syncTimeout (default 10s) elapses, returning context.DeadlineExceeded.
//	That raw error propagates as:
//	    "error in Wait sync switchover reconcile phase: context deadline exceeded"
//	and the cluster never advances – the slave-reconnect fix added to
//	waitForReplicaSync (commit 9dc14190) does not run on this path.
//
//	Fix: at the top of waitForNewPrimarySync, treat "no slave configured" as
//	"no relay log events to wait for" – set replicasSynced=true and return nil.
//	Phase 4 will reconfigure the primary cleanly on the same iteration.
//
//	Because this path requires live SQL connections it is covered by integration
//	tests.  This comment serves as the design record.
func TestWaitForNewPrimarySync_NoSlaveSkipsRelayLogWait(_ *testing.T) {
	// Integration-test only; documented here for design record.
	// See pkg/controller/replication/switchover.go: waitForNewPrimarySync.
}

// TestConfigurePrimary_IdempotentAfterPartialFailure documents the bug:
//
//	ConfigurePrimary chains StopAllSlaves -> ResetAllSlaves -> ResetGtidSlavePos
//	inside an `if isReplica` gate.  If ResetAllSlaves succeeds but ResetGtidSlavePos
//	fails (transient TLS / context error on the same connection), the pod is left
//	with no slave entry but a preserved gtid_slave_pos.
//
//	On retry, IsReplicationReplica returns false (slave entry gone), the entire
//	StopAllSlaves/ResetAllSlaves/ResetGtidSlavePos block is skipped, and
//	gtid_slave_pos stays polluted.  ConfigurePrimary then returns success despite
//	leaving the pod in a half-configured state, which interacts badly with the
//	downstream switchover phases and the failover-sync path.
//
//	Fix: align gtid_slave_pos with the pod's own gtid_binlog_pos when they diverge.
//	This handles three cases under gtid_strict_mode=1:
//	  - slave_pos == binlog_pos: no-op (already aligned).
//	  - binlog_pos empty: ResetGtidSlavePos to '' (strict mode is satisfied — no
//	    binlog domains to reconcile).
//	  - binlog_pos non-empty and slave_pos differs: SetGtidSlavePos(binlog_pos).
//	    This satisfies strict mode (slave_pos covers the binlog domains) without
//	    failing on the healthy-promotion case where the new primary's own binlog has
//	    at least an initial GTID_LIST event from its server_id.  An earlier blanket
//	    reset to '' regressed every healthy primary because Error 1948 rejected the
//	    reset whenever the binlog had any GTID for any domain.
//	    gtid_slave_pos is only consulted when a slave thread runs, so freezing it at
//	    the binlog pos is harmless for a pod that is becoming primary.
//
//	Because this path requires live SQL connections it is covered by integration
//	tests.  This comment serves as the design record.
func TestConfigurePrimary_IdempotentAfterPartialFailure(_ *testing.T) {
	// Integration-test only; documented here for design record.
	// See pkg/controller/replication/config.go: ConfigurePrimary.
}

// TestCurrentPrimaryReady_ToleratesTransientConnectBlips documents the bug:
//
//	currentPrimaryReady opens a fresh SQL client to the current primary with a
//	1-second connect budget.  The DSN-level timeout covers TCP dial + TLS handshake
//	+ auth.  On TLS-enabled clusters under operator CPU pressure or network jitter,
//	a 1s budget is too tight and intermittently returns false even when the primary
//	is healthy.
//
//	Each false flips the switchover state machine onto the failover-style sync path
//	(waitForNewPrimarySync), which can leave the new primary's slave thread in a
//	state from which the switchover cannot recover.
//
//	Fix: bump the connect budget to 5s, matching BuildDSN's default for unspecified
//	timeouts.  This still bails out fast on a genuinely-down primary but tolerates
//	transient TLS handshake jitter.
//
//	Because this path requires live SQL connections it is covered by integration
//	tests.  This comment serves as the design record.
func TestCurrentPrimaryReady_ToleratesTransientConnectBlips(_ *testing.T) {
	// Integration-test only; documented here for design record.
	// See pkg/controller/replication/switchover.go: currentPrimaryReady.
}

// TestReplicationReconcile_RefreshesRolesBeforeSwitchover documents the bug:
//
//	Replication.Reconcile previously short-circuited directly to reconcileSwitchover when
//	IsReplicationSwitchoverRequired() was true (spec.podIndex != status.podIndex), bypassing
//	reconcileReplication entirely. That dispatch assumed status.replication.roles was accurate.
//
//	After a node/pod outage in which both pods briefly lose their replication threads,
//	getReplicationRoles (mariadb_controller_status.go) records each pod as
//	ReplicationRoleUnknown (it is set when neither IsReplicationReplica nor
//	HasConnectedReplicas returns true).  If a switchover is then requested, the wedge
//	is:
//	  - reconcileSwitchover -> reconcileStaleSwitchover: not stale (IsSwitchingPrimary
//	    && !IsReplicationSwitchoverRequired = true && !true = false) -> returns nil.
//	  - reconcileSwitchover -> shouldReconcileSwitchover: HasConfiguredReplica()=false
//	    (no role == Replica in status) -> returns false -> reconcile exits silently.
//	  - reconcileReplication, the path that would have refreshed roles by calling
//	    ConfigurePrimary / ConfigureReplica via ReconcileReplicationInPod, was never
//	    invoked because the dispatch took the switchover-only branch.
//	  - Status reconciler still sees Unknown roles (no SQL slave thread on either pod),
//	    so the next reconcile observes the same state.  Stuck forever; condition stays
//	    PrimarySwitched=False with the original error message frozen.
//
//	Fix: always run reconcileReplication first, then reconcileSwitchover.
//	shouldReconcileReplication already permits this during a switchover (it returns
//	(zero, nil) when IsSwitchingPrimary, signaling "OK to proceed"), so the existing
//	safety boundaries around per-pod reconciliation are unchanged.  The per-pod
//	reconciler restores Primary / Replica roles, after which the next reconcile pass
//	finds HasConfiguredReplica()=true and the switchover phases run.
//
//	Because this path requires live SQL connections it is covered by integration
//	tests.  This comment serves as the design record.
func TestReplicationReconcile_RefreshesRolesBeforeSwitchover(_ *testing.T) {
	// Integration-test only; documented here for design record.
	// See pkg/controller/replication/controller.go: Replication.Reconcile dispatch order.
}
