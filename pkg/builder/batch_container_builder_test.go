package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestJobContainerSecurityContext(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	cmd := command.NewCommand([]string{"mariadbd"}, []string{})
	image := "mariadb:10.6"
	volumeMounts := []corev1.VolumeMount{}
	envVar := []corev1.EnvVar{}
	resources := &corev1.ResourceRequirements{}
	mariadb := &mariadbv1alpha1.MariaDB{}
	var securityContext *corev1.SecurityContext

	container, err := builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}

	securityContext = &corev1.SecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	}
	container, err = builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext == nil {
		t.Error("expected SecurityContext not to be nil")
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

	container, err = builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}
}
