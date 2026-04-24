package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
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
