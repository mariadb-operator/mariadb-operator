package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestObservedReplicationRole(t *testing.T) {
	testCases := map[string]struct {
		isReplica              bool
		hasConnectedReplicas   bool
		podIndex               int
		currentPrimaryPodIndex *int
		want                   mariadbv1alpha1.ReplicationRole
	}{
		"replica stays replica": {
			isReplica:              true,
			hasConnectedReplicas:   false,
			podIndex:               0,
			currentPrimaryPodIndex: ptr.To(0),
			want:                   mariadbv1alpha1.ReplicationRoleReplica,
		},
		"current primary without connected replicas stays primary": {
			isReplica:              false,
			hasConnectedReplicas:   false,
			podIndex:               1,
			currentPrimaryPodIndex: ptr.To(1),
			want:                   mariadbv1alpha1.ReplicationRolePrimary,
		},
		"non current primary with connected replicas is primary": {
			isReplica:              false,
			hasConnectedReplicas:   true,
			podIndex:               0,
			currentPrimaryPodIndex: ptr.To(1),
			want:                   mariadbv1alpha1.ReplicationRolePrimary,
		},
		"non replica without connected replicas is unknown": {
			isReplica:              false,
			hasConnectedReplicas:   false,
			podIndex:               0,
			currentPrimaryPodIndex: ptr.To(1),
			want:                   mariadbv1alpha1.ReplicationRoleUnknown,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := observedReplicationRole(
				tc.isReplica,
				tc.hasConnectedReplicas,
				tc.podIndex,
				tc.currentPrimaryPodIndex,
			)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
