package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExporterImagePullSecrets(t *testing.T) {
	builder := newTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-metrics-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
		{
			name: "Secrets in Exporter",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							PodTemplate: mariadbv1alpha1.PodTemplate{
								ImagePullSecrets: []corev1.LocalObjectReference{
									{
										Name: "exporter-registry",
									},
								},
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "exporter-registry",
				},
			},
		},
		{
			name: "Secrets in MariaDB and Exporter",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							PodTemplate: mariadbv1alpha1.PodTemplate{
								ImagePullSecrets: []corev1.LocalObjectReference{
									{
										Name: "exporter-registry",
									},
								},
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "exporter-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildExporterDeployment(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Deployment: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}
