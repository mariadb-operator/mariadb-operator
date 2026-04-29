package replication

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestShouldSkipPrimaryConfiguration(t *testing.T) {
	tests := []struct {
		name                     string
		role                     mariadbv1alpha1.ReplicationRole
		hasConfiguredReplication bool
		want                     bool
	}{
		{
			name:                     "primary before replication configured",
			role:                     mariadbv1alpha1.ReplicationRolePrimary,
			hasConfiguredReplication: false,
			want:                     false,
		},
		{
			name:                     "primary after replication configured",
			role:                     mariadbv1alpha1.ReplicationRolePrimary,
			hasConfiguredReplication: true,
			want:                     true,
		},
		{
			name:                     "replica after replication configured",
			role:                     mariadbv1alpha1.ReplicationRoleReplica,
			hasConfiguredReplication: true,
			want:                     false,
		},
		{
			name:                     "unknown after replication configured",
			role:                     mariadbv1alpha1.ReplicationRoleUnknown,
			hasConfiguredReplication: true,
			want:                     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipPrimaryConfiguration(tt.role, tt.hasConfiguredReplication)
			if got != tt.want {
				t.Fatalf("unexpected skip decision: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestShouldSkipReplicaConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		readOnly      bool
		readOnlyErr   error
		replicaStatus *mariadbv1alpha1.ReplicaStatusVars
		replicaErr    error
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
			wantSkip: true,
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
			}

			got, err := shouldSkipReplicaConfiguration(context.Background(), client, logr.Discard())
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
	replicaStatus *mariadbv1alpha1.ReplicaStatusVars
	replicaErr    error
}

func (f *fakeReplicaStateClient) IsSystemVariableEnabled(context.Context, string) (bool, error) {
	return f.readOnly, f.readOnlyErr
}

func (f *fakeReplicaStateClient) ReplicaStatus(context.Context, logr.Logger) (*mariadbv1alpha1.ReplicaStatusVars, error) {
	return f.replicaStatus, f.replicaErr
}
