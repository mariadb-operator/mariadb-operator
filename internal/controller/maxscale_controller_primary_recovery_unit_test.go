package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"k8s.io/utils/ptr"
)

func TestHasStaleMonitorTopology(t *testing.T) {
	newMariaDB := func(roles map[string]mariadbv1alpha1.ReplicationRole) *mariadbv1alpha1.MariaDB {
		return &mariadbv1alpha1.MariaDB{
			Spec: mariadbv1alpha1.MariaDBSpec{
				Replication: &mariadbv1alpha1.Replication{
					Enabled: true,
				},
			},
			Status: mariadbv1alpha1.MariaDBStatus{
				CurrentPrimary: ptr.To("db-1"),
				Replication: &mariadbv1alpha1.ReplicationStatus{
					Roles: roles,
				},
			},
		}
	}
	newMaxScale := func(servers ...mariadbv1alpha1.MaxScaleServerStatus) *mariadbv1alpha1.MaxScale {
		return &mariadbv1alpha1.MaxScale{
			Status: mariadbv1alpha1.MaxScaleStatus{
				Servers: servers,
			},
		}
	}
	healthyRoles := map[string]mariadbv1alpha1.ReplicationRole{
		"db-0": mariadbv1alpha1.ReplicationRolePrimary,
		"db-1": mariadbv1alpha1.ReplicationRoleReplica,
	}

	testCases := map[string]struct {
		maxscale *mariadbv1alpha1.MaxScale
		mariadb  *mariadbv1alpha1.MariaDB
		want     bool
	}{
		"observed primary is not Master in the pool": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Maintenance, Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: newMariaDB(healthyRoles),
			want:    true,
		},
		"pool holds a Master": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Master, Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: newMariaDB(healthyRoles),
			want:    false,
		},
		"observed primary is down in the pool": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Down"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: newMariaDB(healthyRoles),
			want:    false,
		},
		"no observed primary": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Running"},
			),
			mariadb: newMariaDB(map[string]mariadbv1alpha1.ReplicationRole{
				"db-0": mariadbv1alpha1.ReplicationRoleUnknown,
				"db-1": mariadbv1alpha1.ReplicationRoleUnknown,
			}),
			want: false,
		},
		"switchover in progress is left alone": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Maintenance, Running"},
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-1", State: "Slave, Running"},
			),
			mariadb: func() *mariadbv1alpha1.MariaDB {
				mdb := newMariaDB(healthyRoles)
				condition.SetPrimarySwitching(&mdb.Status, "db-0")
				return mdb
			}(),
			want: false,
		},
		"replication disabled": {
			maxscale: newMaxScale(
				mariadbv1alpha1.MaxScaleServerStatus{Name: "db-0", State: "Running"},
			),
			mariadb: &mariadbv1alpha1.MariaDB{},
			want:    false,
		},
		"nil MariaDB": {
			maxscale: newMaxScale(),
			mariadb:  nil,
			want:     false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := hasStaleMonitorTopology(tc.maxscale, tc.mariadb); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
