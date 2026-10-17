package sql

import (
	"context"
	"testing"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
)

func TestWaitForMariaDBServiceHealthy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := discoveryv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding EndpointSlice scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Port:     3306,
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
	}
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-primary",
			Namespace: mariadb.Namespace,
			Labels: map[string]string{
				metadata.KubernetesServiceLabel: mariadb.PrimaryServiceKey().Name,
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, endpointSlice).
		Build()

	result, err := waitForMariaDB(context.Background(), fakeClient, mariadb, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected zero result, got %+v", result)
	}
}

func TestWaitForMariaDBServiceUnhealthy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := discoveryv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding EndpointSlice scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Port:     3306,
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
	}
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-primary",
			Namespace: mariadb.Namespace,
			Labels: map[string]string{
				metadata.KubernetesServiceLabel: mariadb.PrimaryServiceKey().Name,
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(false),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mariadb, endpointSlice).
		Build()

	result, err := waitForMariaDB(context.Background(), fakeClient, mariadb, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("expected 1s requeue, got %+v", result)
	}
}

type fakeReadOnlyClient struct {
	readOnly bool
	err      error
	calls    int
}

func (f *fakeReadOnlyClient) IsSystemVariableEnabled(context.Context, string) (bool, error) {
	f.calls++
	return f.readOnly, f.err
}

func TestTargetIsReadOnly(t *testing.T) {
	replicated := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{Replication: &mariadbv1alpha1.Replication{Enabled: true}},
	}
	standalone := &mariadbv1alpha1.MariaDB{}

	tests := []struct {
		name      string
		mdb       *mariadbv1alpha1.MariaDB
		client    *fakeReadOnlyClient
		want      bool
		wantCalls int
	}{
		{
			name:      "demoted primary still behind the primary Service",
			mdb:       replicated,
			client:    &fakeReadOnlyClient{readOnly: true},
			want:      true,
			wantCalls: 1,
		},
		{name: "writable primary", mdb: replicated, client: &fakeReadOnlyClient{readOnly: false}, want: false, wantCalls: 1},
		{name: "standalone MariaDB is never checked", mdb: standalone, client: &fakeReadOnlyClient{readOnly: true}, want: false, wantCalls: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := targetIsReadOnly(context.Background(), tt.mdb, tt.client)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want || tt.client.calls != tt.wantCalls {
				t.Fatalf("got read-only %t after %d checks, want %t after %d", got, tt.client.calls, tt.want, tt.wantCalls)
			}
		})
	}
}
