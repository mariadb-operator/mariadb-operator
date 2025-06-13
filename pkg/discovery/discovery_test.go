package discovery

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDiscoveryServiceMonitors(t *testing.T) {
	testDiscoveryResource(t,
		"ServiceMonitors",
		"monitoring.coreos.com/v1",
		"servicemonitors",
		func(d *Discovery) (bool, error) {
			return d.ServiceMonitorExist()
		})
}

func TestDiscoveryCertificates(t *testing.T) {
	testDiscoveryResource(t,
		"Certificates",
		"cert-manager.io/v1",
		"certificates",
		func(d *Discovery) (bool, error) {
			return d.CertificateExist()
		})
}

func TestDiscoveryVolumeSnapshots(t *testing.T) {
	testDiscoveryResource(t,
		"VolumeSnapshots",
		"snapshot.storage.k8s.io/v1",
		"volumesnapshots",
		func(d *Discovery) (bool, error) {
			return d.VolumeSnapshotExist()
		})
}

func TestDiscoverySecurityContextConstraints(t *testing.T) {
	testDiscoveryResource(t,
		"SecurityContextConstraints",
		"security.openshift.io/v1",
		"securitycontextconstraints",
		func(d *Discovery) (bool, error) {
			return d.SecurityContextConstraintsExist()
		})
}

func testDiscoveryResource(t *testing.T, name, group, kind string, discoveryFn func(d *Discovery) (bool, error)) {
	resource := &metav1.APIResourceList{
		GroupVersion: group,
		APIResources: []metav1.APIResource{
			{
				Name: kind,
			},
		},
	}
	discovery, err := NewFakeDiscovery(resource)
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

	discovery, err = NewFakeDiscovery()
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
