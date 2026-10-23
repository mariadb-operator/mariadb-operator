package replication

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetMaxLagSecondsFallsBackToLastKnownValue(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	key := types.NamespacedName{Name: "mdb", Namespace: "default"}
	mdb := &mariadbv1alpha1.MariaDB{}
	mdb.Name, mdb.Namespace = key.Name, key.Namespace
	mdb.Spec.Replication = &mariadbv1alpha1.Replication{
		ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
			Replica: mariadbv1alpha1.ReplicaReplication{MaxLagSeconds: ptr.To(10)},
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mdb).Build()
	p := &ReplicationProbe{mariadbKey: key, k8sClient: k8sClient, readinessLogger: logr.Discard()}

	if got := p.getMaxLagSeconds(context.Background()); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}

	// Simulate an unreachable Kubernetes API.
	p.k8sClient = ctrlclient.NewNamespacedClient(fake.NewClientBuilder().WithScheme(scheme).Build(), "default")
	if got := p.getMaxLagSeconds(context.Background()); got != 10 {
		t.Fatalf("expected cached 10 on error, got %d", got)
	}
}
