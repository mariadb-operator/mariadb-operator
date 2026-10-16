package replication

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	conditions "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShouldSkipPrimaryConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		readOnly     bool
		readOnlyErr  error
		isReplica    bool
		isReplicaErr error
		wantSkip     bool
		wantErr      bool
	}{
		{
			name:     "writable primary without replica status",
			readOnly: false,
			wantSkip: true,
		},
		{
			name:     "read only primary must be reconfigured",
			readOnly: true,
		},
		{
			name:      "primary with replica status must be reconfigured",
			readOnly:  false,
			isReplica: true,
		},
		{
			name:     "writable primary with empty replica status skips",
			readOnly: false,
			wantSkip: true,
		},
		{
			name:        "read only error is returned",
			readOnlyErr: errors.New("read_only unavailable"),
			wantErr:     true,
		},
		{
			name:         "replica status error is returned",
			readOnly:     false,
			isReplicaErr: errors.New("replica status unavailable"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeReplicaStateClient{
				readOnly:     tt.readOnly,
				readOnlyErr:  tt.readOnlyErr,
				isReplica:    tt.isReplica,
				isReplicaErr: tt.isReplicaErr,
			}

			got, err := shouldSkipPrimaryConfiguration(context.Background(), client, logr.Discard())
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSkip {
				t.Fatalf("unexpected skip decision: got %t, want %t", got, tt.wantSkip)
			}
		})
	}
}

func TestShouldSkipReplicaConfiguration(t *testing.T) {
	expectedMasterHost := "mariadb-0.mariadb-internal.default.svc.cluster.local"

	tests := []struct {
		name          string
		readOnly      bool
		readOnlyErr   error
		replicaStatus *mariadbv1alpha1.ReplicaStatusVars
		replicaErr    error
		masterHost    string
		masterHostErr error
		wantSkip      bool
		wantErr       bool
	}{
		{
			name:     "read only with running replica threads",
			readOnly: true,
			replicaStatus: &mariadbv1alpha1.ReplicaStatusVars{
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(true),
			},
			masterHost: expectedMasterHost,
			wantSkip:   true,
		},
		{
			name:     "replica following unexpected primary must be reconfigured",
			readOnly: true,
			replicaStatus: &mariadbv1alpha1.ReplicaStatusVars{
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(true),
			},
			masterHost: "mariadb-1.mariadb-internal.default.svc.cluster.local",
			wantSkip:   false,
		},
		{
			name:     "master host error is returned",
			readOnly: true,
			replicaStatus: &mariadbv1alpha1.ReplicaStatusVars{
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(true),
			},
			masterHostErr: errors.New("master host unavailable"),
			wantErr:       true,
		},
		{
			name:     "writable replica must be reconfigured",
			readOnly: false,
			replicaStatus: &mariadbv1alpha1.ReplicaStatusVars{
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(true),
			},
			wantSkip: false,
		},
		{
			name:     "stopped replica thread must be reconfigured",
			readOnly: true,
			replicaStatus: &mariadbv1alpha1.ReplicaStatusVars{
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(false),
			},
			wantSkip: false,
		},
		{
			name:          "missing replica status must be reconfigured",
			readOnly:      true,
			replicaStatus: nil,
			wantSkip:      false,
		},
		{
			name:        "read only error is returned",
			readOnlyErr: errors.New("read_only unavailable"),
			wantErr:     true,
		},
		{
			name:       "replica status error is returned",
			readOnly:   true,
			replicaErr: errors.New("replica status unavailable"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeReplicaStateClient{
				readOnly:      tt.readOnly,
				readOnlyErr:   tt.readOnlyErr,
				replicaStatus: tt.replicaStatus,
				replicaErr:    tt.replicaErr,
				masterHost:    tt.masterHost,
				masterHostErr: tt.masterHostErr,
			}

			got, err := shouldSkipReplicaConfiguration(context.Background(), client, expectedMasterHost, logr.Discard())
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSkip {
				t.Fatalf("unexpected skip decision: got %t, want %t", got, tt.wantSkip)
			}
		})
	}
}

type fakeReplicaStateClient struct {
	readOnly      bool
	readOnlyErr   error
	isReplica     bool
	isReplicaErr  error
	replicaStatus *mariadbv1alpha1.ReplicaStatusVars
	replicaErr    error
	masterHost    string
	masterHostErr error
}

func (f *fakeReplicaStateClient) IsSystemVariableEnabled(context.Context, string) (bool, error) {
	return f.readOnly, f.readOnlyErr
}

func (f *fakeReplicaStateClient) IsReplicationReplica(context.Context) (bool, error) {
	return f.isReplica, f.isReplicaErr
}

func (f *fakeReplicaStateClient) ReplicaStatus(context.Context, logr.Logger) (*mariadbv1alpha1.ReplicaStatusVars, error) {
	return f.replicaStatus, f.replicaErr
}

func (f *fakeReplicaStateClient) ReplicationMasterHost(context.Context) (string, error) {
	return f.masterHost, f.masterHostErr
}

func TestGetReplicaOptsPreservesBinlogsOnPITRDriftRepair(t *testing.T) {
	reconciler := &ReplicationReconciler{}
	req := &ReconcileRequest{
		mariadb: &mariadbv1alpha1.MariaDB{
			Spec: mariadbv1alpha1.MariaDBSpec{
				Replication: &mariadbv1alpha1.Replication{
					Enabled: true,
				},
				PointInTimeRecoveryRef: &mariadbv1alpha1.LocalObjectReference{
					Name: "pitr",
				},
			},
		},
	}

	replicaOpts, err := reconciler.getReplicaOpts(context.Background(), req, nil, "mariadb-1", 1, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts := ConfigureReplicaOpts{
		ResetMaster: true,
	}
	for _, setOpt := range replicaOpts {
		setOpt(&opts)
	}
	if opts.ResetMaster {
		t.Fatal("expected drift repair to preserve binary logs when PITR is enabled")
	}
}

func TestGetReplicaOptsResetsMasterWithoutPITR(t *testing.T) {
	reconciler := &ReplicationReconciler{}
	req := &ReconcileRequest{
		mariadb: &mariadbv1alpha1.MariaDB{},
	}

	replicaOpts, err := reconciler.getReplicaOpts(context.Background(), req, nil, "mariadb-1", 1, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts := ConfigureReplicaOpts{
		ResetMaster: true,
	}
	for _, setOpt := range replicaOpts {
		setOpt(&opts)
	}
	if !opts.ResetMaster {
		t.Fatal("expected drift repair to reset master when PITR is disabled")
	}
}

func TestChooseBackupGtid(t *testing.T) {
	tests := []struct {
		name         string
		recordedGtid string
		derivedGtid  string
		deriveErr    error
		wantGtid     string
		wantMismatch bool
	}{
		{
			name:         "derivation matches recorded",
			recordedGtid: "0-11-436570",
			derivedGtid:  "0-11-436570",
			wantGtid:     "0-11-436570",
		},
		{
			name:         "poisoned recorded gtid overridden by derived",
			recordedGtid: "0-10-575055",
			derivedGtid:  "0-11-435466",
			wantGtid:     "0-11-435466",
			wantMismatch: true,
		},
		{
			name:         "derivation empty falls back to recorded",
			recordedGtid: "0-11-436570",
			derivedGtid:  "",
			wantGtid:     "0-11-436570",
		},
		{
			name:         "derivation error falls back to recorded",
			recordedGtid: "0-11-436570",
			derivedGtid:  "0-11-1",
			deriveErr:    errors.New("binlog rotated"),
			wantGtid:     "0-11-436570",
		},
		{
			name:         "multi domain derived gtid",
			recordedGtid: "0-99-999999999",
			derivedGtid:  "0-10-192,1-10-4",
			wantGtid:     "0-10-192,1-10-4",
			wantMismatch: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gtid, mismatch := chooseBackupGtid(tc.recordedGtid, tc.derivedGtid, tc.deriveErr)
			if gtid != tc.wantGtid {
				t.Errorf("gtid mismatch: want=%q got=%q", tc.wantGtid, gtid)
			}
			if mismatch != tc.wantMismatch {
				t.Errorf("mismatch flag: want=%v got=%v", tc.wantMismatch, mismatch)
			}
		})
	}
}

func TestShouldReconcilePrimaryDrift(t *testing.T) {
	replicationConfiguredCondition := metav1.Condition{
		Type:   mariadbv1alpha1.ConditionTypeReplicationConfigured,
		Status: metav1.ConditionTrue,
		Reason: mariadbv1alpha1.ConditionReasonReplicationConfigured,
	}
	switchingPrimaryCondition := metav1.Condition{
		Type:   mariadbv1alpha1.ConditionTypePrimarySwitched,
		Status: metav1.ConditionFalse,
		Reason: mariadbv1alpha1.ConditionReasonSwitchPrimary,
	}
	tests := []struct {
		name       string
		replicated bool
		conditions []metav1.Condition
		primary    *int
		want       bool
	}{
		{
			name:       "configured replication with a known primary is repaired",
			replicated: true,
			conditions: []metav1.Condition{replicationConfiguredCondition},
			primary:    ptr.To(1),
			want:       true,
		},
		{
			name:       "replication disabled is skipped",
			conditions: []metav1.Condition{replicationConfiguredCondition},
			primary:    ptr.To(1),
		},
		{
			name:       "replication never configured is skipped",
			replicated: true,
			primary:    ptr.To(1),
		},
		{
			name:       "unknown primary is skipped",
			replicated: true,
			conditions: []metav1.Condition{replicationConfiguredCondition},
		},
		{
			name:       "switchover in progress is skipped",
			replicated: true,
			conditions: []metav1.Condition{replicationConfiguredCondition, switchingPrimaryCondition},
			primary:    ptr.To(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mdb := &mariadbv1alpha1.MariaDB{
				Status: mariadbv1alpha1.MariaDBStatus{
					Conditions:             tt.conditions,
					CurrentPrimaryPodIndex: tt.primary,
				},
			}
			if tt.replicated {
				mdb.Spec.Replication = &mariadbv1alpha1.Replication{
					Enabled: true,
				}
			}

			got := shouldReconcilePrimaryDrift(mdb)
			if got != tt.want {
				t.Fatalf("unexpected drift reconcile decision: got %t, want %t", got, tt.want)
			}
		})
	}
}

type fakeGtidPositionClient struct {
	gtid string
	err  error
}

func (f *fakeGtidPositionClient) GtidCurrentPos(context.Context) (string, error) {
	return f.gtid, f.err
}

func configuredReplicationMariaDB(pitr bool) *mariadbv1alpha1.MariaDB {
	mdb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
	}
	if pitr {
		mdb.Spec.PointInTimeRecoveryRef = &mariadbv1alpha1.LocalObjectReference{Name: "pitr"}
	}
	conditions.SetReplicationConfigured(&mdb.Status)
	return mdb
}

func TestRejoinReplicaOpts(t *testing.T) {
	const demotedPrimaryPos = "0-11-22825293"
	tests := []struct {
		name            string
		mdb             *mariadbv1alpha1.MariaDB
		client          *fakeGtidPositionClient
		wantErr         bool
		wantResetMaster bool
		wantSlavePos    *string
		wantResetSlave  bool
		wantChangeGtid  string
	}{
		{
			name:            "replication never configured resets the bootstrap binary log",
			mdb:             &mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{Replication: &mariadbv1alpha1.Replication{Enabled: true}}},
			client:          &fakeGtidPositionClient{gtid: "0-10-3"},
			wantResetMaster: true,
		},
		{
			name:            "demoted primary resumes from its own position and keeps its binary log",
			mdb:             configuredReplicationMariaDB(false),
			client:          &fakeGtidPositionClient{gtid: demotedPrimaryPos},
			wantResetMaster: false,
			wantSlavePos:    ptr.To(demotedPrimaryPos),
			wantChangeGtid:  "slave_pos",
		},
		{
			name:            "member without any GTID history starts from an empty position",
			mdb:             configuredReplicationMariaDB(false),
			client:          &fakeGtidPositionClient{gtid: ""},
			wantResetMaster: false,
			wantResetSlave:  true,
			wantChangeGtid:  "slave_pos",
		},
		{
			name:            "point in time recovery keeps the binary log",
			mdb:             configuredReplicationMariaDB(true),
			client:          &fakeGtidPositionClient{gtid: demotedPrimaryPos},
			wantResetMaster: false,
			wantSlavePos:    ptr.To(demotedPrimaryPos),
			wantChangeGtid:  "slave_pos",
		},
		{
			name:    "unreadable position is an error, nothing is reset",
			mdb:     configuredReplicationMariaDB(false),
			client:  &fakeGtidPositionClient{err: errors.New("connection refused")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicaOpts, err := rejoinReplicaOpts(context.Background(), tt.mdb, tt.client, "mariadb-1", logr.Discard())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			opts := ConfigureReplicaOpts{ResetMaster: true}
			for _, setOpt := range replicaOpts {
				setOpt(&opts)
			}
			if opts.ResetMaster != tt.wantResetMaster {
				t.Fatalf("unexpected ResetMaster: got %t, want %t", opts.ResetMaster, tt.wantResetMaster)
			}
			if ptr.Deref(opts.GtidSlavePos, "<nil>") != ptr.Deref(tt.wantSlavePos, "<nil>") {
				t.Fatalf("unexpected gtid_slave_pos: got %s, want %s",
					ptr.Deref(opts.GtidSlavePos, "<nil>"), ptr.Deref(tt.wantSlavePos, "<nil>"))
			}
			if opts.ResetGtidSlavePos != tt.wantResetSlave {
				t.Fatalf("unexpected ResetGtidSlavePos: got %t, want %t", opts.ResetGtidSlavePos, tt.wantResetSlave)
			}
			changeMaster := sql.ChangeMasterOpts{}
			for _, setOpt := range opts.ChangeMasterOpts {
				setOpt(&changeMaster)
			}
			if changeMaster.Gtid != tt.wantChangeGtid {
				t.Fatalf("unexpected CHANGE MASTER GTID mode: got %q, want %q", changeMaster.Gtid, tt.wantChangeGtid)
			}
		})
	}
}

func TestShouldVerifyPodState(t *testing.T) {
	unconfigured := &mariadbv1alpha1.MariaDB{}
	configured := configuredReplicationMariaDB(false)
	var missing mariadbv1alpha1.ReplicationRole

	tests := []struct {
		name        string
		mdb         *mariadbv1alpha1.MariaDB
		role        mariadbv1alpha1.ReplicationRole
		wantPrimary bool
		wantReplica bool
	}{
		{
			name:        "role missing after an unreachable status pass is verified live",
			mdb:         configured,
			role:        missing,
			wantPrimary: true,
			wantReplica: true,
		},
		{
			name:        "observed primary is verified live",
			mdb:         configured,
			role:        mariadbv1alpha1.ReplicationRolePrimary,
			wantPrimary: true,
			wantReplica: true,
		},
		{
			name:        "replica being promoted is always configured as primary",
			mdb:         configured,
			role:        mariadbv1alpha1.ReplicationRoleReplica,
			wantPrimary: false,
			wantReplica: true,
		},
		{
			name:        "first configuration is never skipped",
			mdb:         unconfigured,
			role:        missing,
			wantPrimary: false,
			wantReplica: false,
		},
		{
			name:        "observed replica before first configuration completes is verified live",
			mdb:         unconfigured,
			role:        mariadbv1alpha1.ReplicationRoleReplica,
			wantPrimary: false,
			wantReplica: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldVerifyPrimaryState(tt.mdb, tt.role); got != tt.wantPrimary {
				t.Fatalf("unexpected primary verification: got %t, want %t", got, tt.wantPrimary)
			}
			if got := shouldVerifyReplicaState(tt.mdb, tt.role); got != tt.wantReplica {
				t.Fatalf("unexpected replica verification: got %t, want %t", got, tt.wantReplica)
			}
		})
	}
}
