package replication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPromotionCandidateForClient(t *testing.T) {
	gtid := "0-10-100"
	tests := []struct {
		name     string
		client   *fakePromotionCandidateSQLClient
		want     bool
		wantGTID string
		wantPod  string
	}{
		{
			name: "read only synced replica is candidate",
			client: &fakePromotionCandidateSQLClient{
				readOnly: true,
				status: &mariadbv1alpha1.ReplicaStatusVars{
					SlaveIORunning:  ptr.To(true),
					SlaveSQLRunning: ptr.To(true),
					GtidIOPos:       &gtid,
					GtidCurrentPos:  &gtid,
				},
				gtidDomainID: ptr.To[uint32](0),
			},
			want:     true,
			wantGTID: "0-10-100",
			wantPod:  "mariadb-1",
		},
		{
			name: "writable replica is not candidate",
			client: &fakePromotionCandidateSQLClient{
				readOnly: false,
				status: &mariadbv1alpha1.ReplicaStatusVars{
					SlaveIORunning:  ptr.To(true),
					SlaveSQLRunning: ptr.To(true),
					GtidIOPos:       &gtid,
					GtidCurrentPos:  &gtid,
				},
				gtidDomainID: ptr.To[uint32](0),
			},
		},
		{
			name: "unsynced replica is not candidate",
			client: &fakePromotionCandidateSQLClient{
				readOnly: true,
				status: &mariadbv1alpha1.ReplicaStatusVars{
					SlaveIORunning:  ptr.To(true),
					SlaveSQLRunning: ptr.To(true),
					GtidIOPos:       ptr.To("0-10-100"),
					GtidCurrentPos:  ptr.To("0-10-100,0-11-10"),
				},
				gtidDomainID: ptr.To[uint32](0),
			},
		},
		{
			name: "stopped SQL thread is not candidate",
			client: &fakePromotionCandidateSQLClient{
				readOnly: true,
				status: &mariadbv1alpha1.ReplicaStatusVars{
					SlaveIORunning:  ptr.To(true),
					SlaveSQLRunning: ptr.To(false),
					GtidIOPos:       &gtid,
					GtidCurrentPos:  &gtid,
				},
				gtidDomainID: ptr.To[uint32](0),
			},
		},
		{
			name: "read only error is not candidate",
			client: &fakePromotionCandidateSQLClient{
				readOnlyErr: errors.New("read_only unavailable"),
			},
		},
	}

	handler := &FailoverHandler{
		logger: logr.Discard(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.promotionCandidateForClient(context.Background(), "mariadb-1", tt.client, logr.Discard())
			if !tt.want {
				if got != nil {
					t.Fatalf("expected no candidate, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected candidate, got nil")
			}
			if got.name != tt.wantPod {
				t.Fatalf("expected pod %s, got %s", tt.wantPod, got.name)
			}
			if got.gtidCurrentPos.String() != tt.wantGTID {
				t.Fatalf("expected GTID %s, got %s", tt.wantGTID, got.gtidCurrentPos.String())
			}
		})
	}
}

type fakePromotionCandidateSQLClient struct {
	readOnly     bool
	readOnlyErr  error
	status       *mariadbv1alpha1.ReplicaStatusVars
	statusErr    error
	gtidDomainID *uint32
	gtidErr      error
}

func (f *fakePromotionCandidateSQLClient) IsSystemVariableEnabled(context.Context, string) (bool, error) {
	return f.readOnly, f.readOnlyErr
}

func (f *fakePromotionCandidateSQLClient) ReplicaStatus(context.Context, logr.Logger) (*mariadbv1alpha1.ReplicaStatusVars, error) {
	return f.status, f.statusErr
}

func (f *fakePromotionCandidateSQLClient) GtidDomainId(context.Context) (*uint32, error) {
	return f.gtidDomainID, f.gtidErr
}

func TestDivergedReplica(t *testing.T) {
	divergedSince := metav1.NewTime(time.Date(2026, 10, 14, 21, 37, 0, 0, time.UTC))
	mdb := &mariadbv1alpha1.MariaDB{
		Status: mariadbv1alpha1.MariaDBStatus{
			Replication: &mariadbv1alpha1.ReplicationStatus{
				Replicas: map[string]mariadbv1alpha1.ReplicaStatus{
					"mariadb-1": {GtidDelta: ptr.To[uint64](22000), DivergedSince: &divergedSince},
					"mariadb-2": {GtidDelta: ptr.To[uint64](3)},
				},
			},
		},
	}
	tests := []struct {
		pod  string
		want bool
	}{
		{pod: "mariadb-1", want: true},
		{pod: "mariadb-2", want: false},
		{pod: "mariadb-3", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.pod, func(t *testing.T) {
			if got, _ := divergedReplica(mdb, tt.pod); got != tt.want {
				t.Fatalf("unexpected divergence for %s: got %t, want %t", tt.pod, got, tt.want)
			}
		})
	}
	if diverged, _ := divergedReplica(&mariadbv1alpha1.MariaDB{}, "mariadb-1"); diverged {
		t.Fatal("expected no divergence without replication status")
	}
}

func TestPromotedReplicaForClient(t *testing.T) {
	tests := []struct {
		name     string
		client   *fakePromotedReplicaSQLClient
		want     bool
		wantGTID string
	}{
		{
			name: "writable Pod without replication is promoted",
			client: &fakePromotedReplicaSQLClient{
				gtidCurrentPos: "0-11-105",
				gtidDomainID:   ptr.To[uint32](0),
			},
			want:     true,
			wantGTID: "0-11-105",
		},
		{
			name: "read only Pod is not promoted",
			client: &fakePromotedReplicaSQLClient{
				readOnly:       true,
				gtidCurrentPos: "0-11-105",
				gtidDomainID:   ptr.To[uint32](0),
			},
		},
		{
			name: "writable Pod still replicating is not promoted",
			client: &fakePromotedReplicaSQLClient{
				replica:        true,
				gtidCurrentPos: "0-11-105",
				gtidDomainID:   ptr.To[uint32](0),
			},
		},
		{
			name: "replication state error is not promoted",
			client: &fakePromotedReplicaSQLClient{
				replicaErr:     errors.New("connection refused"),
				gtidCurrentPos: "0-11-105",
				gtidDomainID:   ptr.To[uint32](0),
			},
		},
		{
			name: "empty GTID position is not promoted",
			client: &fakePromotedReplicaSQLClient{
				gtidDomainID: ptr.To[uint32](0),
			},
		},
	}

	handler := &FailoverHandler{
		logger: logr.Discard(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.promotedReplicaForClient(context.Background(), "mariadb-1", tt.client, logr.Discard())
			if !tt.want {
				if got != nil {
					t.Fatalf("expected no promoted replica, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected promoted replica, got nil")
			}
			if got.name != "mariadb-1" {
				t.Fatalf("expected pod mariadb-1, got %s", got.name)
			}
			if got.gtidCurrentPos.String() != tt.wantGTID {
				t.Fatalf("expected GTID %s, got %s", tt.wantGTID, got.gtidCurrentPos.String())
			}
		})
	}
}

func TestPrimaryLossCandidate(t *testing.T) {
	candidate := func(name, gtid string) promotionCandidate {
		parsed, err := replication.ParseGtid(gtid)
		if err != nil {
			t.Fatalf("unexpected error parsing GTID %s: %v", gtid, err)
		}
		return promotionCandidate{name: name, gtidCurrentPos: parsed}
	}
	tests := []struct {
		name       string
		promoted   []promotionCandidate
		candidates []promotionCandidate
		want       string
		wantErr    bool
	}{
		{
			name:       "replica already promoted by MaxScale",
			promoted:   []promotionCandidate{candidate("mariadb-1", "0-11-105")},
			candidates: nil,
			want:       "mariadb-1",
		},
		{
			name:       "promoted replica wins over a replica replicating from it",
			promoted:   []promotionCandidate{candidate("mariadb-1", "0-11-105")},
			candidates: []promotionCandidate{candidate("mariadb-2", "0-11-105")},
			want:       "mariadb-1",
		},
		{
			name:       "promoted replica behind another replica is refused",
			promoted:   []promotionCandidate{candidate("mariadb-1", "0-10-90")},
			candidates: []promotionCandidate{candidate("mariadb-2", "0-10-100")},
			wantErr:    true,
		},
		{
			name: "two promoted replicas are refused",
			promoted: []promotionCandidate{
				candidate("mariadb-1", "0-11-105"),
				candidate("mariadb-2", "0-12-106"),
			},
			wantErr: true,
		},
		{
			name: "no promoted replica falls back to the furthest advanced",
			candidates: []promotionCandidate{
				candidate("mariadb-1", "0-10-99"),
				candidate("mariadb-2", "0-10-100"),
			},
			want: "mariadb-2",
		},
		{
			name:    "no candidate at all",
			wantErr: true,
		},
	}

	handler := &FailoverHandler{
		logger: logr.Discard(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handler.primaryLossCandidate(tt.promoted, tt.candidates)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got candidate %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected candidate %s, got %s", tt.want, got)
			}
		})
	}
}

type fakePromotedReplicaSQLClient struct {
	readOnly       bool
	readOnlyErr    error
	replica        bool
	replicaErr     error
	gtidCurrentPos string
	gtidPosErr     error
	gtidDomainID   *uint32
	gtidErr        error
}

func (f *fakePromotedReplicaSQLClient) IsSystemVariableEnabled(context.Context, string) (bool, error) {
	return f.readOnly, f.readOnlyErr
}

func (f *fakePromotedReplicaSQLClient) IsReplicationReplica(context.Context) (bool, error) {
	return f.replica, f.replicaErr
}

func (f *fakePromotedReplicaSQLClient) GtidCurrentPos(context.Context) (string, error) {
	return f.gtidCurrentPos, f.gtidPosErr
}

func (f *fakePromotedReplicaSQLClient) GtidDomainId(context.Context) (*uint32, error) {
	return f.gtidDomainID, f.gtidErr
}
