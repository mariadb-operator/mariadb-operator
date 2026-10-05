package replication

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func newSwitchoverMariaDB(currentPrimary, newPrimary int, switching bool,
	roles map[string]mariadbv1alpha1.ReplicationRole) *mariadbv1alpha1.MariaDB {
	mdb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-repl",
			Namespace: "default",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(newPrimary),
					},
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimaryPodIndex: ptr.To(currentPrimary),
		},
	}
	if roles != nil {
		mdb.Status.Replication = &mariadbv1alpha1.ReplicationStatus{
			Roles: roles,
		}
	}
	if switching {
		mdb.Status.Conditions = []metav1.Condition{
			{
				Type:   mariadbv1alpha1.ConditionTypePrimarySwitched,
				Status: metav1.ConditionFalse,
				Reason: mariadbv1alpha1.ConditionReasonSwitchPrimary,
			},
		}
	}
	return mdb
}

func TestSwitchoverCandidate(t *testing.T) {
	mdb := newSwitchoverMariaDB(0, 1, false, nil)
	if got := switchoverCandidate(mdb); got != "mariadb-repl-1" {
		t.Fatalf("unexpected switchover candidate: got %q, want %q", got, "mariadb-repl-1")
	}
}

func TestShouldReconcileSwitchover(t *testing.T) {
	primaryAndReplica := map[string]mariadbv1alpha1.ReplicationRole{
		"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
		"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleReplica,
	}
	primaryAndUnknown := map[string]mariadbv1alpha1.ReplicationRole{
		"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
		"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleUnknown,
	}
	tests := []struct {
		name string
		mdb  *mariadbv1alpha1.MariaDB
		want bool
	}{
		{
			name: "no switchover required",
			mdb:  newSwitchoverMariaDB(0, 0, false, primaryAndReplica),
		},
		{
			name: "MaxScale owns the switchover",
			mdb: func() *mariadbv1alpha1.MariaDB {
				mdb := newSwitchoverMariaDB(0, 1, false, primaryAndReplica)
				mdb.Spec.MaxScaleRef = &mariadbv1alpha1.ObjectReference{Name: "maxscale"}
				return mdb
			}(),
		},
		{
			name: "new primary is a configured replica",
			mdb:  newSwitchoverMariaDB(0, 1, false, primaryAndReplica),
			want: true,
		},
		{
			name: "new primary role is unknown",
			mdb:  newSwitchoverMariaDB(0, 1, false, primaryAndUnknown),
		},
		{
			name: "new primary is missing from the observed roles",
			mdb: newSwitchoverMariaDB(0, 1, false, map[string]mariadbv1alpha1.ReplicationRole{
				"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
			}),
		},
		{
			name: "another Pod is the only configured replica",
			mdb: newSwitchoverMariaDB(0, 1, false, map[string]mariadbv1alpha1.ReplicationRole{
				"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
				"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleUnknown,
				"mariadb-repl-2": mariadbv1alpha1.ReplicationRoleReplica,
			}),
		},
		{
			name: "replication status not observed yet",
			mdb:  newSwitchoverMariaDB(0, 1, false, nil),
		},
		{
			name: "in-flight switchover resumes after promoting the only replica",
			mdb:  newSwitchoverMariaDB(0, 1, true, primaryAndUnknown),
			want: true,
		},
		{
			name: "in-flight switchover resumes without observed roles",
			mdb:  newSwitchoverMariaDB(0, 1, true, nil),
			want: true,
		},
		{
			name: "in-flight switchover with a configured replica",
			mdb:  newSwitchoverMariaDB(0, 1, true, primaryAndReplica),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReconcileSwitchover(tt.mdb); got != tt.want {
				t.Fatalf("unexpected switchover reconcile decision: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsSwitchoverPostponed(t *testing.T) {
	primaryAndReplica := map[string]mariadbv1alpha1.ReplicationRole{
		"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
		"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleReplica,
	}
	primaryAndUnknown := map[string]mariadbv1alpha1.ReplicationRole{
		"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
		"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleUnknown,
	}
	tests := []struct {
		name string
		mdb  *mariadbv1alpha1.MariaDB
		want bool
	}{
		{
			name: "no switchover required",
			mdb:  newSwitchoverMariaDB(0, 0, false, primaryAndUnknown),
		},
		{
			name: "new primary is a configured replica",
			mdb:  newSwitchoverMariaDB(0, 1, false, primaryAndReplica),
		},
		{
			name: "new primary is not a configured replica",
			mdb:  newSwitchoverMariaDB(0, 1, false, primaryAndUnknown),
			want: true,
		},
		{
			name: "replication status not observed yet",
			mdb:  newSwitchoverMariaDB(0, 1, false, nil),
			want: true,
		},
		{
			name: "switchover already in flight",
			mdb:  newSwitchoverMariaDB(0, 1, true, primaryAndUnknown),
		},
		{
			name: "MaxScale owns the switchover",
			mdb: func() *mariadbv1alpha1.MariaDB {
				mdb := newSwitchoverMariaDB(0, 1, false, primaryAndUnknown)
				mdb.Spec.MaxScaleRef = &mariadbv1alpha1.ObjectReference{Name: "maxscale"}
				return mdb
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSwitchoverPostponed(tt.mdb); got != tt.want {
				t.Fatalf("unexpected postponed switchover decision: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSwitchoverAbandonReason(t *testing.T) {
	roles := map[string]mariadbv1alpha1.ReplicationRole{
		"mariadb-repl-0": mariadbv1alpha1.ReplicationRolePrimary,
		"mariadb-repl-1": mariadbv1alpha1.ReplicationRoleUnknown,
	}
	withReplicaToRecover := func(mdb *mariadbv1alpha1.MariaDB, pod string) *mariadbv1alpha1.MariaDB {
		mdb.Status.Replication.ReplicaToRecover = ptr.To(pod)
		return mdb
	}
	tests := []struct {
		name      string
		mdb       *mariadbv1alpha1.MariaDB
		podExists bool
		want      string
	}{
		{
			name:      "switchover not started",
			mdb:       newSwitchoverMariaDB(0, 1, false, roles),
			podExists: false,
		},
		{
			name:      "stale switchover is not abandoned",
			mdb:       newSwitchoverMariaDB(0, 0, true, roles),
			podExists: false,
		},
		{
			name:      "new primary Pod present and not recovering",
			mdb:       newSwitchoverMariaDB(0, 1, true, roles),
			podExists: true,
		},
		{
			name:      "new primary Pod missing",
			mdb:       newSwitchoverMariaDB(0, 1, true, roles),
			podExists: false,
			want:      "Pod 'mariadb-repl-1' does not exist",
		},
		{
			name:      "new primary under replica recovery",
			mdb:       withReplicaToRecover(newSwitchoverMariaDB(0, 1, true, roles), "mariadb-repl-1"),
			podExists: true,
			want:      "Pod 'mariadb-repl-1' is under replica recovery",
		},
		{
			name:      "another Pod under replica recovery",
			mdb:       withReplicaToRecover(newSwitchoverMariaDB(0, 1, true, roles), "mariadb-repl-0"),
			podExists: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := switchoverAbandonReason(tt.mdb, tt.podExists); got != tt.want {
				t.Fatalf("unexpected abandon reason: got %q, want %q", got, tt.want)
			}
		})
	}
}
