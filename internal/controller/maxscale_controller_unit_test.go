package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestAutoMaintenanceRequired(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimary: ptr.To("db-0"),
			Replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"db-0": mariadbv1alpha1.ReplicationRolePrimary,
					"db-1": mariadbv1alpha1.ReplicationRoleReplica,
				},
			},
		},
	}

	testCases := map[string]struct {
		serverName string
		mariadb    *mariadbv1alpha1.MariaDB
		want       bool
	}{
		"primary stays online": {
			serverName: "db-0",
			mariadb:    mariadb,
			want:       false,
		},
		"configured replica stays online": {
			serverName: "db-1",
			mariadb:    mariadb,
			want:       false,
		},
		"unknown replica is held in maintenance": {
			serverName: "db-2",
			mariadb:    mariadb,
			want:       true,
		},
		"missing primary keeps servers in maintenance": {
			serverName: "db-0",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			want: true,
		},
		"non replication leaves maintenance unchanged": {
			serverName: "db-0",
			mariadb:    &mariadbv1alpha1.MariaDB{},
			want:       false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := autoMaintenanceRequired(tc.serverName, tc.mariadb); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestEffectiveMaxScaleServerPreservesManualMaintenance(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimary: ptr.To("db-0"),
			Replication: &mariadbv1alpha1.ReplicationStatus{
				Roles: map[string]mariadbv1alpha1.ReplicationRole{
					"db-0": mariadbv1alpha1.ReplicationRolePrimary,
				},
			},
		},
	}

	server := effectiveMaxScaleServer(mariadbv1alpha1.MaxScaleServer{
		Name:        "db-0",
		Maintenance: true,
	}, mariadb)

	if !server.Maintenance {
		t.Fatalf("expected manual maintenance to be preserved")
	}
}
