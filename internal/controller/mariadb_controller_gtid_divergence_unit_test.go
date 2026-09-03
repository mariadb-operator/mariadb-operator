package controller

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGtidDelta(t *testing.T) {
	logger := log.Log

	testCases := map[string]struct {
		primary    primaryGtidState
		replicaPos *string
		want       *uint64
	}{
		"replica behind the primary": {
			primary: primaryGtidState{
				currentPos: ptr.To("0-11-8338752"),
				domainID:   ptr.To(uint32(0)),
			},
			replicaPos: ptr.To("0-11-1268397"),
			want:       ptr.To(uint64(7070355)),
		},
		"replica in sync": {
			primary: primaryGtidState{
				currentPos: ptr.To("0-11-8338752"),
				domainID:   ptr.To(uint32(0)),
			},
			replicaPos: ptr.To("0-11-8338752"),
			want:       ptr.To(uint64(0)),
		},
		"replica ahead of the primary": {
			primary: primaryGtidState{
				currentPos: ptr.To("0-11-8338752"),
				domainID:   ptr.To(uint32(0)),
			},
			replicaPos: ptr.To("0-11-8338800"),
			want:       ptr.To(uint64(0)),
		},
		"multiple domains uses the replication domain": {
			primary: primaryGtidState{
				currentPos: ptr.To("1-10-500,0-11-8338752"),
				domainID:   ptr.To(uint32(0)),
			},
			replicaPos: ptr.To("1-10-9000,0-11-8338700"),
			want:       ptr.To(uint64(52)),
		},
		"unknown primary position": {
			primary:    primaryGtidState{domainID: ptr.To(uint32(0))},
			replicaPos: ptr.To("0-11-1268397"),
			want:       nil,
		},
		"unknown domain ID": {
			primary:    primaryGtidState{currentPos: ptr.To("0-11-8338752")},
			replicaPos: ptr.To("0-11-1268397"),
			want:       nil,
		},
		"unknown replica position": {
			primary: primaryGtidState{
				currentPos: ptr.To("0-11-8338752"),
				domainID:   ptr.To(uint32(0)),
			},
			replicaPos: nil,
			want:       nil,
		},
		"replication domain missing in replica position": {
			primary: primaryGtidState{
				currentPos: ptr.To("0-11-8338752"),
				domainID:   ptr.To(uint32(0)),
			},
			replicaPos: ptr.To("1-10-500"),
			want:       nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := gtidDelta(tc.primary, tc.replicaPos, logger)
			if !ptr.Equal(got, tc.want) {
				t.Fatalf("expected %v, got %v", ptr.Deref(tc.want, 0), ptr.Deref(got, 0))
			}
		})
	}
}

func TestSetGtidDivergence(t *testing.T) {
	now := time.Date(2026, time.September, 3, 6, 41, 0, 0, time.UTC)
	earlier := metav1.NewTime(now.Add(-10 * time.Minute))

	testCases := map[string]struct {
		status            *mariadbv1alpha1.ReplicaStatus
		delta             *uint64
		maxDelta          uint64
		wantDelta         *uint64
		wantDivergedSince *metav1.Time
	}{
		"delta below threshold clears divergence": {
			status:            &mariadbv1alpha1.ReplicaStatus{DivergedSince: ptr.To(earlier)},
			delta:             ptr.To(uint64(10)),
			maxDelta:          1000,
			wantDelta:         ptr.To(uint64(10)),
			wantDivergedSince: nil,
		},
		"delta at threshold clears divergence": {
			status:            &mariadbv1alpha1.ReplicaStatus{DivergedSince: ptr.To(earlier)},
			delta:             ptr.To(uint64(1000)),
			maxDelta:          1000,
			wantDelta:         ptr.To(uint64(1000)),
			wantDivergedSince: nil,
		},
		"delta above threshold starts the divergence clock": {
			status:            &mariadbv1alpha1.ReplicaStatus{},
			delta:             ptr.To(uint64(7070355)),
			maxDelta:          1000,
			wantDelta:         ptr.To(uint64(7070355)),
			wantDivergedSince: ptr.To(metav1.NewTime(now)),
		},
		"sustained divergence keeps the original instant": {
			status:            &mariadbv1alpha1.ReplicaStatus{DivergedSince: ptr.To(earlier)},
			delta:             ptr.To(uint64(7070355)),
			maxDelta:          1000,
			wantDelta:         ptr.To(uint64(7070355)),
			wantDivergedSince: ptr.To(earlier),
		},
		"zero threshold disables divergence detection": {
			status:            &mariadbv1alpha1.ReplicaStatus{DivergedSince: ptr.To(earlier)},
			delta:             ptr.To(uint64(7070355)),
			maxDelta:          0,
			wantDelta:         ptr.To(uint64(7070355)),
			wantDivergedSince: nil,
		},
		"unknown delta keeps the last observation": {
			status: &mariadbv1alpha1.ReplicaStatus{
				GtidDelta:     ptr.To(uint64(7070355)),
				DivergedSince: ptr.To(earlier),
			},
			delta:             nil,
			maxDelta:          1000,
			wantDelta:         ptr.To(uint64(7070355)),
			wantDivergedSince: ptr.To(earlier),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			setGtidDivergence(tc.status, tc.delta, tc.maxDelta, now)

			if !ptr.Equal(tc.status.GtidDelta, tc.wantDelta) {
				t.Fatalf("expected delta %v, got %v", ptr.Deref(tc.wantDelta, 0), ptr.Deref(tc.status.GtidDelta, 0))
			}
			if tc.wantDivergedSince == nil {
				if tc.status.DivergedSince != nil {
					t.Fatalf("expected divergence to be cleared, got %v", tc.status.DivergedSince)
				}
				return
			}
			if tc.status.DivergedSince == nil {
				t.Fatalf("expected divergence at %v, got nil", tc.wantDivergedSince)
			}
			if !tc.status.DivergedSince.Equal(tc.wantDivergedSince) {
				t.Fatalf("expected divergence at %v, got %v", tc.wantDivergedSince, tc.status.DivergedSince)
			}
		})
	}
}
