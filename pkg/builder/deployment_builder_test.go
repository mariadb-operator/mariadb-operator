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

func TestExporterMaxScaleImagePullSecrets(t *testing.T) {
	builder := newTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-metrics-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		maxscale        *mariadbv1alpha1.MaxScale
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
					},
				},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MaxScale",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "maxscale-registry",
							},
						},
					},
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "maxscale-registry",
				},
			},
		},
		{
			name: "Secrets in MaxScale",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
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
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "maxscale-registry",
							},
						},
					},
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
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
					Name: "maxscale-registry",
				},
				{
					Name: "exporter-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildMaxScaleExporterDeployment(tt.maxscale, &mariadbv1alpha1.MariaDB{})
			if err != nil {
				t.Fatalf("unexpected error building Deployment: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestExporterDeploymentMeta(t *testing.T) {
	builder := newTestBuilder()
	mdbObjMeta := metav1.ObjectMeta{
		Name: "test",
	}
	tests := []struct {
		name           string
		mariadb        *mariadbv1alpha1.MariaDB
		wantDeployMeta *mariadbv1alpha1.Metadata
		wantPodMeta    *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			wantDeployMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantDeployMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							PodTemplate: mariadbv1alpha1.PodTemplate{
								PodMetadata: &mariadbv1alpha1.Metadata{
									Labels: map[string]string{
										"database.myorg.io": "pod",
									},
									Annotations: map[string]string{
										"database.myorg.io": "pod",
									},
								},
							},
						},
					},
				},
			},
			wantDeployMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "pod",
				},
				Annotations: map[string]string{
					"database.myorg.io": "pod",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							PodTemplate: mariadbv1alpha1.PodTemplate{
								PodMetadata: &mariadbv1alpha1.Metadata{
									Labels: map[string]string{
										"database.myorg.io": "pod",
									},
									Annotations: map[string]string{
										"database.myorg.io": "pod",
									},
								},
							},
						},
					},
				},
			},
			wantDeployMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "pod",
				},
				Annotations: map[string]string{
					"database.myorg.io": "pod",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploy, err := builder.BuildExporterDeployment(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building Deployment: %v", err)
			}
			assertObjectMeta(t, &deploy.ObjectMeta, tt.wantDeployMeta.Labels, tt.wantDeployMeta.Annotations)
			assertObjectMeta(t, &deploy.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}
