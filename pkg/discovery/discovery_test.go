package discovery

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeClient "k8s.io/client-go/kubernetes/fake"
)

func TestDiscoveryServiceMonitors(t *testing.T) {
	testDiscovery(t, "ServiceMonitors", "monitoring.coreos.com/v1", "servicemonitors", func(d *Discovery) (bool, error) {
		return d.ServiceMonitorExist()
	})
}

func TestDiscoverySecurityContextConstraints(t *testing.T) {
	testDiscovery(t, "SecurityContextConstraints", "security.openshift.io/v1", "securitycontextconstraints", func(d *Discovery) (bool, error) {
		return d.SecurityContextConstrainstsExist()
	})
}

func testDiscovery(t *testing.T, name, group, kind string, discoveryFn func(d *Discovery) (bool, error)) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: group,
			APIResources: []metav1.APIResource{
				{
					Name: kind,
				},
			},
		},
	}
	discovery, err := newFakeDiscovery(resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, err := discoveryFn(discovery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("expected to have discovered '%s'", name)
	}

	discovery, err = newFakeDiscovery(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, err = discoveryFn(discovery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Errorf("expected to not have discovered '%s'", name)
	}
}

func newFakeDiscovery(resources []*metav1.APIResourceList) (*Discovery, error) {
	client := fakeClient.NewSimpleClientset()
	fakeDiscovery := client.Discovery().(*fakeDiscovery.FakeDiscovery)
	fakeDiscovery.Resources = resources
	return NewDiscoveryWithClient(fakeDiscovery)
}
