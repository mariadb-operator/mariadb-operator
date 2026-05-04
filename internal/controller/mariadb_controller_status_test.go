package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

// TestReplicationRoleFor pins the role-resolution behaviour that
// getReplicationRoles depends on.  The configured-primary fallback (case 4
// below) is the post-fix behaviour: without it, a primary whose only replica
// is unreachable falls through to Unknown and wedges the operator into a
// silent ConfigurePrimary loop (see field incident hotcrp-ltichiring3-db,
// 32h wedge, 217/251 operator log lines = "Configuring primary" with zero
// errors).
func TestReplicationRoleFor(t *testing.T) {
	mdbWithPrimary := func(idx int) *mariadbv1alpha1.MariaDB {
		return &mariadbv1alpha1.MariaDB{
			Status: mariadbv1alpha1.MariaDBStatus{
				CurrentPrimaryPodIndex: ptr.To(idx),
			},
		}
	}
	mdbNoPrimary := &mariadbv1alpha1.MariaDB{}

	tests := []struct {
		name                 string
		mdb                  *mariadbv1alpha1.MariaDB
		podIndex             int
		isReplica            bool
		hasConnectedReplicas bool
		want                 mariadbv1alpha1.ReplicationRole
	}{
		{
			name:                 "replica wins regardless of other signals",
			mdb:                  mdbWithPrimary(0),
			podIndex:             1,
			isReplica:            true,
			hasConnectedReplicas: false,
			want:                 mariadbv1alpha1.ReplicationRoleReplica,
		},
		{
			name:                 "configured primary that has somehow become a replica is reported as Replica",
			mdb:                  mdbWithPrimary(0),
			podIndex:             0,
			isReplica:            true,
			hasConnectedReplicas: false,
			want:                 mariadbv1alpha1.ReplicationRoleReplica,
		},
		{
			name:                 "primary by connected-replicas signal",
			mdb:                  mdbWithPrimary(0),
			podIndex:             0,
			isReplica:            false,
			hasConnectedReplicas: true,
			want:                 mariadbv1alpha1.ReplicationRolePrimary,
		},
		{
			name:                 "configured primary with no connected replicas falls back to Primary (regression guard for ltichiring3 wedge)",
			mdb:                  mdbWithPrimary(0),
			podIndex:             0,
			isReplica:            false,
			hasConnectedReplicas: false,
			want:                 mariadbv1alpha1.ReplicationRolePrimary,
		},
		{
			name:                 "non-primary index with no signals is Unknown",
			mdb:                  mdbWithPrimary(0),
			podIndex:             1,
			isReplica:            false,
			hasConnectedReplicas: false,
			want:                 mariadbv1alpha1.ReplicationRoleUnknown,
		},
		{
			name:                 "no configured primary and no signals is Unknown (no fallback to apply)",
			mdb:                  mdbNoPrimary,
			podIndex:             0,
			isReplica:            false,
			hasConnectedReplicas: false,
			want:                 mariadbv1alpha1.ReplicationRoleUnknown,
		},
		{
			name:                 "no configured primary still resolves Primary via connected-replicas signal",
			mdb:                  mdbNoPrimary,
			podIndex:             0,
			isReplica:            false,
			hasConnectedReplicas: true,
			want:                 mariadbv1alpha1.ReplicationRolePrimary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replicationRoleFor(tt.mdb, tt.podIndex, tt.isReplica, tt.hasConnectedReplicas)
			if got != tt.want {
				t.Errorf("replicationRoleFor(podIndex=%d, isReplica=%v, hasConnectedReplicas=%v) = %q, want %q",
					tt.podIndex, tt.isReplica, tt.hasConnectedReplicas, got, tt.want)
			}
		})
	}
}
