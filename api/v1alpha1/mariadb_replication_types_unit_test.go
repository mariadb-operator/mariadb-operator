package v1alpha1

import (
	"testing"

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
