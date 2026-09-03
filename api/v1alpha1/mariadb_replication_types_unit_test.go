package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestReplicaStatusVarsIOThreadStates(t *testing.T) {
	tests := []struct {
		name        string
		status      *ReplicaStatusVars
		wantRunning bool
		wantActive  bool
	}{
		{
			name:   "nil status",
			status: nil,
		},
		{
			name:   "empty status",
			status: &ReplicaStatusVars{},
		},
		{
			name: "io thread running",
			status: &ReplicaStatusVars{
				SlaveIORunning: ptr.To(true),
				SlaveIOState:   ptr.To("Yes"),
			},
			wantRunning: true,
			wantActive:  true,
		},
		{
			name: "io thread connecting",
			status: &ReplicaStatusVars{
				SlaveIORunning: ptr.To(false),
				SlaveIOState:   ptr.To(ReplicaIOStateConnecting),
			},
			wantRunning: false,
			wantActive:  true,
		},
		{
			name: "io thread stopped",
			status: &ReplicaStatusVars{
				SlaveIORunning: ptr.To(false),
				SlaveIOState:   ptr.To("No"),
			},
		},
		{
			name: "legacy status without state",
			status: &ReplicaStatusVars{
				SlaveIORunning: ptr.To(true),
			},
			wantRunning: true,
			wantActive:  true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.IsIOThreadRunning(); got != tc.wantRunning {
				t.Errorf("IsIOThreadRunning mismatch: want=%v got=%v", tc.wantRunning, got)
			}
			if got := tc.status.IsIOThreadActive(); got != tc.wantActive {
				t.Errorf("IsIOThreadActive mismatch: want=%v got=%v", tc.wantActive, got)
			}
		})
	}
}

func TestReplicaStatusVarsEqualErrorsThreadTransitions(t *testing.T) {
	base := &ReplicaStatusVars{
		LastIOErrno:     ptr.To(0),
		LastSQLErrno:    ptr.To(0),
		SlaveIORunning:  ptr.To(true),
		SlaveSQLRunning: ptr.To(true),
	}
	tests := []struct {
		name  string
		other *ReplicaStatusVars
		want  bool
	}{
		{
			name: "identical",
			other: &ReplicaStatusVars{
				LastIOErrno:     ptr.To(0),
				LastSQLErrno:    ptr.To(0),
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(true),
			},
			want: true,
		},
		{
			name: "errno transition",
			other: &ReplicaStatusVars{
				LastIOErrno:     ptr.To(0),
				LastSQLErrno:    ptr.To(1062),
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(true),
			},
			want: false,
		},
		{
			name: "io thread transition without errno",
			other: &ReplicaStatusVars{
				LastIOErrno:     ptr.To(0),
				LastSQLErrno:    ptr.To(0),
				SlaveIORunning:  ptr.To(false),
				SlaveSQLRunning: ptr.To(true),
			},
			want: false,
		},
		{
			name: "sql thread transition without errno",
			other: &ReplicaStatusVars{
				LastIOErrno:     ptr.To(0),
				LastSQLErrno:    ptr.To(0),
				SlaveIORunning:  ptr.To(true),
				SlaveSQLRunning: ptr.To(false),
			},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := base.EqualErrors(tc.other); got != tc.want {
				t.Errorf("EqualErrors mismatch: want=%v got=%v", tc.want, got)
			}
		})
	}
}

func TestMariaDBIsReplicaDiverged(t *testing.T) {
	now := time.Date(2026, time.September, 3, 7, 47, 0, 0, time.UTC)

	newMariaDB := func(replication *Replication, status *ReplicaStatus) *MariaDB {
		mdb := &MariaDB{
			Spec: MariaDBSpec{
				Replication: replication,
			},
		}
		if status != nil {
			mdb.Status.Replication = &ReplicationStatus{
				Replicas: map[string]ReplicaStatus{
					"db-1": *status,
				},
			}
		}
		return mdb
	}
	enabled := &Replication{Enabled: true}

	tests := []struct {
		name    string
		mariadb *MariaDB
		podName string
		want    bool
	}{
		{
			name:    "replication disabled",
			mariadb: newMariaDB(nil, &ReplicaStatus{DivergedSince: ptr.To(metav1.NewTime(now.Add(-1 * time.Hour)))}),
			podName: "db-1",
		},
		{
			name:    "no replication status",
			mariadb: newMariaDB(enabled, nil),
			podName: "db-1",
		},
		{
			name:    "replica not diverged",
			mariadb: newMariaDB(enabled, &ReplicaStatus{GtidDelta: ptr.To(uint64(3))}),
			podName: "db-1",
		},
		{
			name:    "divergence below the duration threshold",
			mariadb: newMariaDB(enabled, &ReplicaStatus{DivergedSince: ptr.To(metav1.NewTime(now.Add(-1 * time.Minute)))}),
			podName: "db-1",
		},
		{
			name:    "divergence above the duration threshold",
			mariadb: newMariaDB(enabled, &ReplicaStatus{DivergedSince: ptr.To(metav1.NewTime(now.Add(-1 * time.Hour)))}),
			podName: "db-1",
			want:    true,
		},
		{
			name:    "unknown replica",
			mariadb: newMariaDB(enabled, &ReplicaStatus{DivergedSince: ptr.To(metav1.NewTime(now.Add(-1 * time.Hour)))}),
			podName: "db-2",
		},
		{
			name: "detection disabled by a zero threshold",
			mariadb: newMariaDB(
				&Replication{
					Enabled: true,
					ReplicationSpec: ReplicationSpec{
						Replica: ReplicaReplication{
							MaxGtidDelta: ptr.To(uint64(0)),
						},
					},
				},
				&ReplicaStatus{DivergedSince: ptr.To(metav1.NewTime(now.Add(-1 * time.Hour)))},
			),
			podName: "db-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mariadb.isReplicaDivergedAt(tt.podName, now); got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestReplicaReplicationGtidDivergenceDefaults(t *testing.T) {
	mdb := &MariaDB{
		Spec: MariaDBSpec{
			Replication: &Replication{
				Enabled: true,
			},
		},
	}
	replica := ReplicaReplication{}
	replica.SetDefaults(mdb)

	if ptr.Deref(replica.MaxGtidDelta, 0) != DefaultMaxGtidDelta {
		t.Errorf("expected max GTID delta %d, got %d", DefaultMaxGtidDelta, ptr.Deref(replica.MaxGtidDelta, 0))
	}
	duration := ptr.Deref(replica.MaxGtidDeltaDuration, metav1.Duration{})
	if duration.Duration != DefaultMaxGtidDeltaDuration {
		t.Errorf("expected max GTID delta duration %s, got %s", DefaultMaxGtidDeltaDuration, duration.Duration)
	}
}

func TestMariaDBObservedPrimary(t *testing.T) {
	newMariaDB := func(roles map[string]ReplicationRole) *MariaDB {
		mdb := &MariaDB{}
		if roles != nil {
			mdb.Status.Replication = &ReplicationStatus{Roles: roles}
		}
		return mdb
	}

	tests := []struct {
		name    string
		mariadb *MariaDB
		want    string
	}{
		{
			name:    "no replication status",
			mariadb: newMariaDB(nil),
		},
		{
			name: "single primary",
			mariadb: newMariaDB(map[string]ReplicationRole{
				"db-0": ReplicationRolePrimary,
				"db-1": ReplicationRoleReplica,
			}),
			want: "db-0",
		},
		{
			name: "no primary",
			mariadb: newMariaDB(map[string]ReplicationRole{
				"db-0": ReplicationRoleUnknown,
				"db-1": ReplicationRoleReplica,
			}),
		},
		{
			name: "ambiguous primary",
			mariadb: newMariaDB(map[string]ReplicationRole{
				"db-0": ReplicationRolePrimary,
				"db-1": ReplicationRolePrimary,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mariadb.ObservedPrimary(); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
