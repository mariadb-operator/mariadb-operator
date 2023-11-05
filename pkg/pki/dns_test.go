package pki

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestServiceDNSNames(t *testing.T) {
	t.Setenv("CLUSTER_NAME", "cluster.test")
	serviceKey := types.NamespacedName{
		Name:      "test",
		Namespace: "test-namespace",
	}

	dnsNames := ServiceDNSNames(serviceKey)
	if dnsNames == nil {
		t.Fatal("Expecting DNS names not to be nil")
	}
	expectedCN := "test.test-namespace.svc"
	if expectedCN != dnsNames.CommonName {
		t.Fatalf("Expected CommonName to be %s. Got %s", expectedCN, dnsNames.CommonName)
	}
	expectedDNSNames := []string{
		"test.test-namespace.svc.cluster.test",
		"test.test-namespace",
		"test",
	}
	if !reflect.DeepEqual(expectedDNSNames, dnsNames.Names) {
		t.Fatalf("Expected DNS names to be %s. Got %s", expectedDNSNames, dnsNames.CommonName)
	}
}
