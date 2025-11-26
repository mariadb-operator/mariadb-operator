package builder

import (
	"slices"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
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

func TestJobS3Env(t *testing.T) {
	tests := []struct {
		name        string
		s3          *mariadbv1alpha1.S3
		expectedEnv []string
	}{
		{
			name:        "nil S3",
			s3:          nil,
			expectedEnv: nil,
		},
		{
			name: "S3 with access key only",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
			},
			expectedEnv: []string{batchS3AccessKeyId, batchS3SecretAccessKey},
		},
		{
			name: "S3 with session token",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SessionTokenSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "session-token",
				},
			},
			expectedEnv: []string{batchS3AccessKeyId, batchS3SecretAccessKey, batchS3SessionTokenKey},
		},
		{
			name: "S3 with SSE-C",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SSEC: &mariadbv1alpha1.SSECConfig{
					CustomerKeySecretKeyRef: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "ssec-key",
						},
						Key: "customer-key",
					},
				},
			},
			expectedEnv: []string{batchS3AccessKeyId, batchS3SecretAccessKey, batchS3SSECCustomerKey},
		},
		{
			name: "S3 with all options",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SessionTokenSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "session-token",
				},
				SSEC: &mariadbv1alpha1.SSECConfig{
					CustomerKeySecretKeyRef: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "ssec-key",
						},
						Key: "customer-key",
					},
				},
			},
			expectedEnv: []string{batchS3AccessKeyId, batchS3SecretAccessKey, batchS3SessionTokenKey, batchS3SSECCustomerKey},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := jobS3Env(tt.s3)

			if tt.expectedEnv == nil {
				if env != nil {
					t.Errorf("expected nil env, got: %v", env)
				}
				return
			}

			if len(env) != len(tt.expectedEnv) {
				t.Errorf("expected %d env vars, got: %d", len(tt.expectedEnv), len(env))
				return
			}

			for _, expectedName := range tt.expectedEnv {
				found := slices.ContainsFunc(env, func(e corev1.EnvVar) bool {
					return e.Name == expectedName
				})
				if !found {
					t.Errorf("expected env var %s not found in %v", expectedName, env)
				}
			}
		})
	}
}
