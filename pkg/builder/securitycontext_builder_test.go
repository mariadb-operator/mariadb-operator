package builder

import (
	"testing"

	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildContainerSecurityContext(t *testing.T) {
	builder := newTestBuilder(t)

	sc, err := builder.buildContainerSecurityContext(&corev1.SecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	})
	if err != nil {
		t.Fatalf("unexpected error building SecurityContext: %v", err)
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
		RunAsUser: ptr.To(mysqlUser),
	})
	if err != nil {
		t.Fatalf("unexpected error building SecurityContext: %v", err)
	}
	if sc != nil {
		t.Error("SecurityContext must be nil")
	}
}

func TestBuildPodSecurityContext(t *testing.T) {
	builder := newTestBuilder(t)

	sc, err := builder.buildPodSecurityContext(&corev1.PodSecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	})
	if err != nil {
		t.Fatalf("unexpected error building PodSecurityContext: %v", err)
	}
	if sc == nil {
		t.Error("PodSecurityContext must be non nil")
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

	sc, err = builder.buildPodSecurityContext(&corev1.PodSecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	})
	if err != nil {
		t.Fatalf("unexpected error building PodSecurityContext: %v", err)
	}
	if sc != nil {
		t.Error("PodSecurityContext must be nil")
	}
}
