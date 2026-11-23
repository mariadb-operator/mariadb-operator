package health

import (
	"context"
	"testing"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsServiceEndpoint(t *testing.T) {
	serviceKey := types.NamespacedName{Name: "mariadb-galera-primary", Namespace: "default"}
	endpointSlice := func(name, serviceName string, podNames ...string) *discoveryv1.EndpointSlice {
		var endpoints []discoveryv1.Endpoint
		for _, podName := range podNames {
			endpoints = append(endpoints, discoveryv1.Endpoint{
				Addresses: []string{"10.0.0.1"},
				TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: podName, Namespace: "default"},
			})
		}
		return &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{metadata.KubernetesServiceLabel: serviceName},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   endpoints,
		}
	}

	tests := []struct {
		name    string
		objects []ctrlclient.Object
		podName string
		want    bool
	}{
		{
			name:    "no EndpointSlices",
			podName: "mariadb-galera-0",
			want:    false,
		},
		{
			name:    "Pod is an endpoint",
			objects: []ctrlclient.Object{endpointSlice("primary", serviceKey.Name, "mariadb-galera-0")},
			podName: "mariadb-galera-0",
			want:    true,
		},
		{
			name:    "another Pod is the endpoint",
			objects: []ctrlclient.Object{endpointSlice("primary", serviceKey.Name, "mariadb-galera-1")},
			podName: "mariadb-galera-0",
			want:    false,
		},
		{
			name:    "Pod is an endpoint of another Service",
			objects: []ctrlclient.Object{endpointSlice("secondary", "mariadb-galera-secondary", "mariadb-galera-0")},
			podName: "mariadb-galera-0",
			want:    false,
		},
		{
			name: "Pod is an endpoint in one of several EndpointSlices",
			objects: []ctrlclient.Object{
				endpointSlice("primary-a", serviceKey.Name, "mariadb-galera-1"),
				endpointSlice("primary-b", serviceKey.Name, "mariadb-galera-0"),
			},
			podName: "mariadb-galera-0",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithObjects(tt.objects...).Build()
			got, err := IsServiceEndpoint(context.Background(), client, serviceKey, tt.podName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
