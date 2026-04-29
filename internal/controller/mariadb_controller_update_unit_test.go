package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetPodsByRoleAllowsSingleReplicaHACluster(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-repl",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 1,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			CurrentPrimary: ptr.To("mariadb-repl-0"),
		},
	}
	primaryPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-repl-0",
			Namespace: "test",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "mariadb",
				"app.kubernetes.io/instance": mariadb.Name,
			},
		},
	}

	reconciler := &MariaDBReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(mariadb, primaryPod).
			Build(),
	}

	var podsByRole podRoleSet
	result, err := reconciler.getPodsByRole(context.Background(), mariadb, &podsByRole, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected zero result, got %+v", result)
	}
	if podsByRole.primary.Name != primaryPod.Name {
		t.Fatalf("unexpected primary pod: %q", podsByRole.primary.Name)
	}
	if len(podsByRole.replicas) != 0 {
		t.Fatalf("expected no replica pods, got %d", len(podsByRole.replicas))
	}
}

func TestSyncMaxScalePrimaryStatusUpdatesStaleMariaDBPrimary(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
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
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, maxscale).
		Build()
	reconciler := &MariaDBReconciler{
		Client:      fakeClient,
		RefResolver: refresolver.New(fakeClient),
	}

	result, err := reconciler.syncMaxScalePrimaryStatus(context.Background(), mariadb, logr.Discard())
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
	if updated.IsSwitchingPrimary() {
		t.Fatal("expected primary switched condition after MaxScale status sync")
	}
}
