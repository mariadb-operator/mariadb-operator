package replication

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestReplicaSQLThreadFailed(t *testing.T) {
	tests := []struct {
		name   string
		status *mariadbv1alpha1.ReplicaStatusVars
		want   bool
	}{
		{
			name:   "running SQL thread",
			status: &mariadbv1alpha1.ReplicaStatusVars{SlaveSQLRunning: ptr.To(true), LastSQLErrno: ptr.To(0)},
			want:   false,
		},
		{
			name:   "SQL thread stopped by mariadb-backup --safe-slave-backup",
			status: &mariadbv1alpha1.ReplicaStatusVars{SlaveSQLRunning: ptr.To(false), LastSQLErrno: ptr.To(0)},
			want:   false,
		},
		{
			name:   "SQL thread stopped by a duplicate key error",
			status: &mariadbv1alpha1.ReplicaStatusVars{SlaveSQLRunning: ptr.To(false), LastSQLErrno: ptr.To(1062)},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replicaSQLThreadFailed(tt.status); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}
