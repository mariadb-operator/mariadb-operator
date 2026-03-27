package controller

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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

func TestSetReadyWithMariaDBWaitsForConfiguredReplica(t *testing.T) {
	mdb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
	}
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	condition.SetReadyWithMariaDB(&mdb.Status, sts, mdb)

	ready := meta.FindStatusCondition(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected ready condition to be set")
	}
	if ready.Status != "False" {
		t.Fatalf("expected ready condition to be false, got %s", ready.Status)
	}
	if ready.Reason != mariadbv1alpha1.ConditionReasonReplicationNotConfigured {
		t.Fatalf("expected reason %s, got %s", mariadbv1alpha1.ConditionReasonReplicationNotConfigured, ready.Reason)
	}
}

func TestSetReadyWithMariaDBAllowsConfiguredReplica(t *testing.T) {
	mdb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
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
	}
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	condition.SetReadyWithMariaDB(&mdb.Status, sts, mdb)

	ready := meta.FindStatusCondition(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected ready condition to be set")
	}
	if ready.Status != "True" {
		t.Fatalf("expected ready condition to be true, got %s", ready.Status)
	}
	if ready.Reason != mariadbv1alpha1.ConditionReasonStatefulSetReady {
		t.Fatalf("expected reason %s, got %s", mariadbv1alpha1.ConditionReasonStatefulSetReady, ready.Reason)
	}
}
