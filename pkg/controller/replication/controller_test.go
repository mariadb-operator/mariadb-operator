package replication

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
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
