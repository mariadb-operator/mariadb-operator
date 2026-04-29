package controller

import (
	"context"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestSetReadyWithMariaDBKeepsPendingUpdateNotReady(t *testing.T) {
	mdb := &mariadbv1alpha1.MariaDB{
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeUpdated,
					Status: "False",
					Reason: mariadbv1alpha1.ConditionReasonPendingUpdate,
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      1,
			ReadyReplicas: 1,
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
	if ready.Reason != mariadbv1alpha1.ConditionReasonPendingUpdate {
		t.Fatalf("expected reason %s, got %s", mariadbv1alpha1.ConditionReasonPendingUpdate, ready.Reason)
	}
}

func TestReconcileStatusRequeuesAfterMaxScalePrimarySync(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding apps scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-cluster",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			MaxScaleRef: &mariadbv1alpha1.ObjectReference{
				Name: "maxscale",
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimary:         ptr.To("db-cluster-1"),
			CurrentPrimaryPodIndex: ptr.To(1),
		},
	}
	maxscale := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maxscale",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			Servers: []mariadbv1alpha1.MaxScaleServer{
				{
					Name:    "db-cluster-0",
					Address: "db-cluster-0.db-cluster-internal.test.svc.cluster.local",
				},
				{
					Name:    "db-cluster-1",
					Address: "db-cluster-1.db-cluster-internal.test.svc.cluster.local",
				},
			},
		},
		Status: mariadbv1alpha1.MaxScaleStatus{
			Servers: []mariadbv1alpha1.MaxScaleServerStatus{
				{
					Name:  "db-cluster-0",
					State: "Master, Running",
				},
				{
					Name:  "db-cluster-1",
					State: "Slave, Running",
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-cluster",
			Namespace: "test",
		},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, maxscale, sts).
		Build()
	reconciler := &MariaDBReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
	}

	result, err := reconciler.reconcileStatus(context.Background(), mariadb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsZero() {
		t.Fatalf("expected requeue after syncing primary status, got %+v", result)
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if updated.Status.CurrentPrimaryPodIndex == nil || *updated.Status.CurrentPrimaryPodIndex != 0 {
		t.Fatalf("expected current primary index 0, got %v", updated.Status.CurrentPrimaryPodIndex)
	}
	if updated.Status.CurrentPrimary == nil || *updated.Status.CurrentPrimary != "db-cluster-0" {
		t.Fatalf("expected current primary db-cluster-0, got %v", updated.Status.CurrentPrimary)
	}
}
