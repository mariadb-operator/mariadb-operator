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
// This test currently FAILS (asserts the desired correct behaviour, which the bug violates).
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
	// It means gtid_slave_pos is left untouched on the replica, which is the right behaviour.
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
