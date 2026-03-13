package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
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
	var securityContext *mariadbv1alpha1.SecurityContext

	container, err := builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}

	securityContext = &mariadbv1alpha1.SecurityContext{
		RunAsUser: ptr.To(mysqlUser),
	}
	container, err = builder.jobContainer("mariadb", cmd, image, volumeMounts, envVar, resources, mariadb, securityContext)
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
	discovery, err := discovery.NewFakeDiscovery(resource)
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

func TestPhysicalBackupJobEnv(t *testing.T) {
	secretKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: "test",
		},
		Key: "test",
	}
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		expected []corev1.EnvVar
	}{
		{
			name: "Environment with credentials",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: secretKeySelector,
						Generate:          false,
					},
				},
			},
			expected: []corev1.EnvVar{
				{Name: batchUserEnv, Value: "root"},
				{
					Name: batchPasswordEnv,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: ptr.To(secretKeySelector.ToKubernetesType()),
					},
				},
			},
		},
		{
			name: "Environment with credentials and additional env",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: secretKeySelector,
						Generate:          false,
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Env: []mariadbv1alpha1.EnvVar{
							{
								Name:  "TEST",
								Value: "TEST",
							},
						},
					},
				},
			},
			expected: []corev1.EnvVar{
				{Name: batchUserEnv, Value: "root"},
				{
					Name: batchPasswordEnv,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: ptr.To(secretKeySelector.ToKubernetesType()),
					},
				},
				{Name: "TEST", Value: "TEST"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := physicalBackupJobEnv(tt.mariadb)

			if len(got) != len(tt.expected) {
				t.Errorf("got %d env vars, want %d", len(got), len(tt.expected))
			}

			// Using reflect.DeepEqual or cmp.Diff to validate the slice content
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("unexpected env vars:\ngot: %v\nwant: %v", got, tt.expected)
			}
		})
	}
}
