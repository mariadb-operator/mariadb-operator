package replication

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
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
