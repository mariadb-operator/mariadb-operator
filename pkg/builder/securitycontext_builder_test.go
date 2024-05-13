package builder

import (
	"testing"

	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildContainerSecurityContext(t *testing.T) {
	builder := newDefaultTestBuilder(t)

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
	discovery, err := discovery.NewFakeDiscovery(false, resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(discovery)

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
	builder := newDefaultTestBuilder(t)

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
	discovery, err := discovery.NewFakeDiscovery(false, resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(discovery)

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

func TestBuildPodSecurityContextWithUserGroup(t *testing.T) {
	builder := newDefaultTestBuilder(t)

	sc, err := builder.buildPodSecurityContextWithUserGroup(&corev1.PodSecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	}, mysqlUser, mysqlGroup)
	if err != nil {
		t.Fatalf("unexpected error building PodSecurityContext: %v", err)
	}
	if sc == nil {
		t.Error("PodSecurityContext must be non nil")
	}

	sc, err = builder.buildPodSecurityContextWithUserGroup(nil, mysqlUser, mysqlGroup)
	if err != nil {
		t.Fatalf("unexpected error building PodSecurityContext: %v", err)
	}
	if sc == nil {
		t.Fatal("PodSecurityContext must be non nil")
	}
	runAsUser := ptr.Deref(sc.RunAsUser, 0)
	if runAsUser != mysqlUser {
		t.Errorf("expected to run as mysql user, got user: %d", runAsUser)
	}
	runAsGroup := ptr.Deref(sc.RunAsGroup, 0)
	if runAsGroup != mysqlGroup {
		t.Errorf("expected to run as mysql group, got group: %d", runAsGroup)
	}
	fsGroup := ptr.Deref(sc.FSGroup, 0)
	if fsGroup != mysqlGroup {
		t.Errorf("expected to run as mysql fsGroup, got fsGroup: %d", fsGroup)
	}

	resource := &metav1.APIResourceList{
		GroupVersion: "security.openshift.io/v1",
		APIResources: []metav1.APIResource{
			{
				Name: "securitycontextconstraints",
			},
		},
	}
	discovery, err := discovery.NewFakeDiscovery(false, resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(discovery)

	sc, err = builder.buildPodSecurityContextWithUserGroup(&corev1.PodSecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	}, mysqlUser, mysqlGroup)
	if err != nil {
		t.Fatalf("unexpected error building PodSecurityContext: %v", err)
	}
	if sc != nil {
		t.Error("PodSecurityContext must be nil")
	}
}
