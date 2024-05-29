package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestMariadbPodMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		opts     []mariadbPodOpt
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			opts: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			opts: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "HA",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			opts: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
		},
		{
			name: "Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			opts: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "extra meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			opts: []mariadbPodOpt{
				withMeta(&mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				}),
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "inherit and Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			opts: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "extra override Pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			opts: []mariadbPodOpt{
				withMeta(&mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
				}),
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "true",
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "Pod override inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			opts: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "without selector labels",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			opts: []mariadbPodOpt{
				withMariadbSelectorLabels(false),
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			opts: []mariadbPodOpt{
				withMeta(&mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				}),
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
					"sidecar.istio.io/inject": "false",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl, err := builder.mariadbPodTemplate(tt.mariadb, tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
			}
			assertObjectMeta(t, &podTpl.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestMaxScalePodMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "maxscale-obj",
	}
	tests := []struct {
		name     string
		maxscale *mariadbv1alpha1.MaxScale
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod meta",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "inherit and Pod meta",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "Pod override inherit meta",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "all",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"k8s.mariadb.com": "test",
						},
						Annotations: map[string]string{
							"k8s.mariadb.com": "test",
						},
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"database.myorg.io": "mariadb",
							},
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
					"k8s.mariadb.com":            "test",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com":   "test",
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl, err := builder.maxscalePodTemplate(tt.maxscale)
			if err != nil {
				t.Fatalf("unexpected error building MaxScale Pod template: %v", err)
			}

			assertObjectMeta(t, &podTpl.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestMariadbPodBuilder(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mariadb-builder",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("300Mi")),
			},
		},
	}
	opts := []mariadbPodOpt{
		withAffinity(&mariadbv1alpha1.AffinityConfig{
			AntiAffinityEnabled: ptr.To(true),
			Affinity:            corev1.Affinity{},
		}),
		withResources(&corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"cpu": resource.MustParse("100m"),
			},
		}),
	}

	podTpl, err := builder.mariadbPodTemplate(mariadb, opts...)
	if err != nil {
		t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
	}
	if podTpl.Spec.Affinity == nil {
		t.Error("expected affinity to have been set")
	}

	if reflect.ValueOf(podTpl.Spec.Containers[0].Resources).IsZero() {
		t.Error("expected resources to have been set")
	}

	if podTpl.Spec.SecurityContext == nil {
		t.Error("expected podSecurityContext to have been set")
	}
	sc := ptr.Deref(podTpl.Spec.SecurityContext, corev1.PodSecurityContext{})
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
}

func TestMaxscalePodBuilder(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mxs := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-maxscale-builder",
		},
	}

	podTpl, err := builder.maxscalePodTemplate(mxs)
	if err != nil {
		t.Fatalf("unexpected error building MaxScale Pod template: %v", err)
	}

	if podTpl.Spec.SecurityContext == nil {
		t.Error("expected podSecurityContext to have been set")
	}
	sc := ptr.Deref(podTpl.Spec.SecurityContext, corev1.PodSecurityContext{})
	runAsUser := ptr.Deref(sc.RunAsUser, 0)
	if runAsUser != maxscaleUser {
		t.Errorf("expected to run as maxscale user, got user: %d", runAsUser)
	}
	runAsGroup := ptr.Deref(sc.RunAsGroup, 0)
	if runAsGroup != maxscaleGroup {
		t.Errorf("expected to run as maxscale group, got group: %d", runAsGroup)
	}
	fsGroup := ptr.Deref(sc.FSGroup, 0)
	if fsGroup != maxscaleGroup {
		t.Errorf("expected to run as maxscale fsGroup, got fsGroup: %d", fsGroup)
	}

	d, err := discovery.NewDiscoveryEnterprise()
	if err != nil {
		t.Fatalf("unexpected error creating discovery: %v", err)
	}
	builder = newTestBuilder(d)

	podTpl, err = builder.maxscalePodTemplate(mxs)
	if err != nil {
		t.Fatalf("unexpected error building MaxScale Pod template: %v", err)
	}

	if podTpl.Spec.SecurityContext == nil {
		t.Error("expected podSecurityContext to have been set")
	}
	sc = ptr.Deref(podTpl.Spec.SecurityContext, corev1.PodSecurityContext{})
	runAsUser = ptr.Deref(sc.RunAsUser, 0)
	if runAsUser != maxscaleEnterpriseUser {
		t.Errorf("expected to run as maxscale user, got user: %d", runAsUser)
	}
	runAsGroup = ptr.Deref(sc.RunAsGroup, 0)
	if runAsGroup != maxscaleEnterpriseGroup {
		t.Errorf("expected to run as maxscale group, got group: %d", runAsGroup)
	}
	fsGroup = ptr.Deref(sc.FSGroup, 0)
	if fsGroup != maxscaleEnterpriseGroup {
		t.Errorf("expected to run as maxscale fsGroup, got fsGroup: %d", fsGroup)
	}
}

func TestMariadbConfigVolume(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mariadb-builder",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("300Mi")),
			},
		},
	}

	volume := mariadbConfigVolume(mariadb)
	if volume.Projected == nil {
		t.Fatal("expected volume to be projected")
	}
	expectedSources := 1
	if len(volume.Projected.Sources) != expectedSources {
		t.Fatalf("expecting to have %d sources, got: %d", expectedSources, len(volume.Projected.Sources))
	}
	expectedKey := "0-default.cnf"
	if volume.Projected.Sources[0].ConfigMap.Items[0].Key != expectedKey {
		t.Fatalf("expecting to have '%s' key, got: '%s'", expectedKey, volume.Projected.Sources[0].ConfigMap.Items[0].Key)
	}

	mariadb.Spec.MyCnfConfigMapKeyRef = &corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "test",
		},
		Key: "my.cnf",
	}

	volume = mariadbConfigVolume(mariadb)
	if volume.Projected == nil {
		t.Fatal("expected volume to be projected")
	}
	expectedSources = 2
	if len(volume.Projected.Sources) != expectedSources {
		t.Fatalf("expecting to have %d sources, got: %d", expectedSources, len(volume.Projected.Sources))
	}
	expectedKey = "0-default.cnf"
	if volume.Projected.Sources[0].ConfigMap.Items[0].Key != expectedKey {
		t.Fatalf("expecting to have '%s' key, got: '%s'", expectedKey, volume.Projected.Sources[0].ConfigMap.Items[0].Key)
	}
	expectedKey = "my.cnf"
	if volume.Projected.Sources[1].ConfigMap.Items[0].Key != expectedKey {
		t.Fatalf("expecting to have '%s' key, got: '%s'", expectedKey, volume.Projected.Sources[0].ConfigMap.Items[0].Key)
	}
}
