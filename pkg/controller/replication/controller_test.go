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

	replicaOpts, err := reconciler.getReplicaOpts(context.Background(), req, "mariadb-1", 1, logr.Discard())
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

	replicaOpts, err := reconciler.getReplicaOpts(context.Background(), req, "mariadb-1", 1, logr.Discard())
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
