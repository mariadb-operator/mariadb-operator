package builder

import (
	"testing"

	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestContainerSecurityBuilder(t *testing.T) {
	builder := newTestBuilder(t)

	sc, err := builder.buildContainerSecurityContext(&corev1.SecurityContext{
		RunAsUser: ptr.To(int64(999)),
	})
	if err != nil {
		t.Fatalf("unexpected error building Security Context: %v", err)
	}
	if sc == nil {
		t.Error("SecurityContext must be non nil")
	}

	resource := &metav1.APIResourceList{
		GroupVersion: "security.openshift.io/v1",
		APIResources: []metav1.APIResource{
			{
				Name: "securitycontextconstraints",
			},
		},
	}
	discovery, err := discovery.NewFakeDiscovery(resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(t, WithDiscovery(discovery))

	sc, err = builder.buildContainerSecurityContext(&corev1.SecurityContext{
		RunAsUser: ptr.To(int64(999)),
	})
	if err != nil {
		t.Fatalf("unexpected error building Security Context: %v", err)
	}
	if sc != nil {
		t.Error("SecurityContext must be nil")
	}
}
