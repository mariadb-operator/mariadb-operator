package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestShouldWaitForConfiguredReplicaBackupTarget(t *testing.T) {
	testCases := map[string]struct {
		backup *mariadbv1alpha1.PhysicalBackup
		mdb    *mariadbv1alpha1.MariaDB
		want   bool
	}{
		"standalone does not wait": {
			backup: &mariadbv1alpha1.PhysicalBackup{},
			mdb:    &mariadbv1alpha1.MariaDB{},
			want:   false,
		},
		"replication without configured replica waits for strict replica target": {
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetReplica),
				},
			},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			want: true,
		},
		"replication with configured replica does not wait": {
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetReplica),
				},
			},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					Replication: &mariadbv1alpha1.ReplicationStatus{
						Roles: map[string]mariadbv1alpha1.ReplicationRole{
							"mariadb-1": mariadbv1alpha1.ReplicationRoleReplica,
						},
					},
				},
			},
			want: false,
		},
		"prefer replica target can fall back to primary": {
			backup: &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				},
			},
			mdb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			want: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := shouldWaitForConfiguredReplicaBackupTarget(tc.backup, tc.mdb)
			if got != tc.want {
				t.Fatalf("expected %t, got %t", tc.want, got)
			}
		})
	}
}
