package replication

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldWaitForPrimaryRole(t *testing.T) {
	replicationConfigured := metav1.Condition{
		Type:   mariadbv1alpha1.ConditionTypeReplicationConfigured,
		Status: metav1.ConditionTrue,
		Reason: mariadbv1alpha1.ConditionReasonReplicationConfigured,
	}
	tests := []struct {
		name            string
		configured      bool
		replication     *mariadbv1alpha1.ReplicationStatus
		primaryPodIndex int
		want            bool
	}{
		{
			name:            "never configured, no replication status",
			configured:      false,
			replication:     nil,
			primaryPodIndex: 0,
			want:            true,
		},
		{
			name:       "never configured, roles only for the replicas",
			configured: false,
			replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleReplica,
					"mariadb-repl-2": mariadbv1alpha1.ReplicationRoleReplica,
				},
			},
			primaryPodIndex: 0,
			want:            true,
		},
		{
			name:       "never configured, primary role unknown but assigned",
			configured: false,
			replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"mariadb-repl-0": mariadbv1alpha1.ReplicationRoleUnknown,
				},
			},
			primaryPodIndex: 0,
			want:            false,
		},
		{
			name:       "never configured, role assigned to the primary of another index",
			configured: false,
			replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"mariadb-repl-0": mariadbv1alpha1.ReplicationRoleReplica,
				},
			},
			primaryPodIndex: 1,
			want:            true,
		},
		{
			name:            "configured, no replication status",
			configured:      true,
			replication:     nil,
			primaryPodIndex: 0,
			want:            false,
		},
		{
			name:       "configured, roles only for the replicas",
			configured: true,
			replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleReplica,
				},
			},
			primaryPodIndex: 0,
			want:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mdb := &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-repl",
					Namespace: "default",
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					Replication: tt.replication,
				},
			}
			if tt.configured {
				meta.SetStatusCondition(&mdb.Status.Conditions, replicationConfigured)
			}
			assert.Equal(t, tt.want, shouldWaitForPrimaryRole(mdb, tt.primaryPodIndex))
		})
	}
}
