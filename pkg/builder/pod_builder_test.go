package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
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
			name: "without HA annotations",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
							Annotations: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			opts: []mariadbPodOpt{
				withHAAnnotations(false),
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
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
					MaxScalePodTemplate: mariadbv1alpha1.MaxScalePodTemplate{
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
					MaxScalePodTemplate: mariadbv1alpha1.MaxScalePodTemplate{
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
					MaxScalePodTemplate: mariadbv1alpha1.MaxScalePodTemplate{
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
					MaxScalePodTemplate: mariadbv1alpha1.MaxScalePodTemplate{
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

func TestMariadbPodBuilderResources(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-resources",
	}
	tests := []struct {
		name          string
		mariadb       *mariadbv1alpha1.MariaDB
		opts          []mariadbPodOpt
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "no resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			opts:          nil,
			wantResources: corev1.ResourceRequirements{},
		},
		{
			name: "mariadb resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("300m"),
							},
						},
					},
				},
			},
			opts: nil,
			wantResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("300m"),
				},
			},
		},
		{
			name: "opt resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			opts: []mariadbPodOpt{
				withResources(&corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu": resource.MustParse("100m"),
					},
				}),
				withMariadbResources(false),
			},
			wantResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		},
		{
			name: "mariadb and opt resources",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("300m"),
							},
						},
					},
				},
			},
			opts: []mariadbPodOpt{
				withResources(&corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu": resource.MustParse("100m"),
					},
				}),
				withMariadbResources(true),
			},
			wantResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
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
			if len(podTpl.Spec.Containers) != 1 {
				t.Error("expecting to have one container")
			}
			resources := podTpl.Spec.Containers[0].Resources
			if !reflect.DeepEqual(resources, tt.wantResources) {
				t.Errorf("unexpected resources, got: %v, expected: %v", resources, tt.wantResources)
			}
		})
	}
}

func TestMariadbPodBuilderServiceAccount(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-serviceaccount",
	}
	tests := []struct {
		name               string
		mariadb            *mariadbv1alpha1.MariaDB
		opts               []mariadbPodOpt
		wantServiceAccount bool
	}{
		{
			name: "serviceaccount",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			opts:               nil,
			wantServiceAccount: true,
		},
		{
			name: "no serviceaccount",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			opts: []mariadbPodOpt{
				withServiceAccount(false),
			},
			wantServiceAccount: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl, err := builder.mariadbPodTemplate(tt.mariadb, tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
			}
			if len(podTpl.Spec.Containers) == 0 {
				t.Error("expecting to have containers")
			}

			container := podTpl.Spec.Containers[0]
			scName := podTpl.Spec.ServiceAccountName
			scVol := datastructures.Find(podTpl.Spec.Volumes, func(vol corev1.Volume) bool {
				return vol.Name == ServiceAccountVolume
			})
			scVolMount := datastructures.Find(container.VolumeMounts, func(volMount corev1.VolumeMount) bool {
				return volMount.Name == ServiceAccountVolume
			})

			if tt.wantServiceAccount {
				if scName != objMeta.Name {
					t.Error("expecting to have ServiceAccount")
				}
				if scVol == nil {
					t.Error("expecting to have ServiceAccount Volume")
				}
				if scVolMount == nil {
					t.Error("expecting to have ServiceAccount VolumeMount")
				}
			} else {
				if scName != "" {
					t.Error("expecting to NOT have ServiceAccount")
				}
				if scVol != nil {
					t.Error("expecting to NOT have ServiceAccount Volume")
				}
				if scVolMount != nil {
					t.Error("expecting to NOT have ServiceAccount VolumeMount")
				}
			}
		})
	}
}

func TestMariadbPodBuilderAffinity(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-affinity",
	}
	tests := []struct {
		name                         string
		mariadb                      *mariadbv1alpha1.MariaDB
		opts                         []mariadbPodOpt
		wantAffinity                 bool
		wantTopologySpreadContraints bool
		wantNodeAffinity             bool
	}{
		{
			name: "no affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts:                         nil,
			wantAffinity:                 false,
			wantTopologySpreadContraints: false,
			wantNodeAffinity:             false,
		},
		{
			name: "mariadb affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							AntiAffinityEnabled: ptr.To(true),
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts:                         nil,
			wantAffinity:                 true,
			wantTopologySpreadContraints: false,
			wantNodeAffinity:             false,
		},
		{
			name: "mariadb topologyspreadconstraints",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						TopologySpreadConstraints: []mariadbv1alpha1.TopologySpreadConstraint{
							{
								MaxSkew:     1,
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts:                         nil,
			wantAffinity:                 false,
			wantTopologySpreadContraints: true,
			wantNodeAffinity:             false,
		},
		{
			name: "opt affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts: []mariadbPodOpt{
				withAffinity(&corev1.Affinity{}),
				withAffinityEnabled(true),
			},
			wantAffinity:                 true,
			wantTopologySpreadContraints: false,
			wantNodeAffinity:             false,
		},
		{
			name: "mariadb and opt affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							AntiAffinityEnabled: ptr.To(true),
						},
						TopologySpreadConstraints: []mariadbv1alpha1.TopologySpreadConstraint{
							{
								MaxSkew:     1,
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts: []mariadbPodOpt{
				withAffinity(&corev1.Affinity{}),
				withAffinityEnabled(true),
			},
			wantAffinity:                 true,
			wantTopologySpreadContraints: true,
			wantNodeAffinity:             false,
		},
		{
			name: "disable affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							AntiAffinityEnabled: ptr.To(true),
						},
						TopologySpreadConstraints: []mariadbv1alpha1.TopologySpreadConstraint{
							{
								MaxSkew:     1,
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts: []mariadbPodOpt{
				withAffinity(&corev1.Affinity{}),
				withAffinityEnabled(false),
			},
			wantAffinity:                 false,
			wantTopologySpreadContraints: false,
			wantNodeAffinity:             false,
		},
		{
			name: "mariadb with node affinity",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							Affinity: mariadbv1alpha1.Affinity{
								NodeAffinity: &mariadbv1alpha1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &mariadbv1alpha1.NodeSelector{
										NodeSelectorTerms: []mariadbv1alpha1.NodeSelectorTerm{
											{
												MatchExpressions: []mariadbv1alpha1.NodeSelectorRequirement{
													{
														Key:      "kubernetes.io/hostname",
														Operator: corev1.NodeSelectorOpIn,
														Values:   []string{"node1", "node2"},
													},
												},
											},
										},
									},
								},
							},
							AntiAffinityEnabled: nil,
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			opts:                         nil,
			wantAffinity:                 true,
			wantTopologySpreadContraints: false,
			wantNodeAffinity:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl, err := builder.mariadbPodTemplate(tt.mariadb, tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
			}
			if tt.wantAffinity && podTpl.Spec.Affinity == nil {
				t.Error("expected affinity to have been set")
			}
			if !tt.wantAffinity && podTpl.Spec.Affinity != nil {
				t.Error("expected affinity to not have been set")
			}

			if tt.wantTopologySpreadContraints && podTpl.Spec.TopologySpreadConstraints == nil {
				t.Error("expected topologySpreadConstraints to have been set")
			}
			if !tt.wantTopologySpreadContraints && podTpl.Spec.TopologySpreadConstraints != nil {
				t.Error("expected topologySpreadConstraints to not have been set")
			}

			if tt.wantNodeAffinity && podTpl.Spec.Affinity.NodeAffinity == nil {
				t.Error("expected node affinity to have been set")
			}
			if !tt.wantNodeAffinity && podTpl.Spec.Affinity != nil && podTpl.Spec.Affinity.NodeAffinity != nil {
				t.Error("expected node affinity to not have been set")
			}
		})
	}
}

func TestMariadbPodBuilderInitContainers(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-initcontainers",
	}
	tests := []struct {
		name               string
		mariadb            *mariadbv1alpha1.MariaDB
		wantInitContainers int
	}{
		{
			name: "no init containers",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					PodTemplate: mariadbv1alpha1.PodTemplate{
						InitContainers: nil,
					},
				},
			},
			wantInitContainers: 0,
		},
		{
			name: "init containers",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					PodTemplate: mariadbv1alpha1.PodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Image: "busybox:latest",
							},
							{
								Image: "busybox:latest",
							},
						},
					},
				},
			},
			wantInitContainers: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl, err := builder.mariadbPodTemplate(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
			}

			if len(podTpl.Spec.InitContainers) != tt.wantInitContainers {
				t.Errorf("unexpected number of init containers, got: %v, want: %v", len(podTpl.Spec.InitContainers), tt.wantInitContainers)
			}

			for _, container := range podTpl.Spec.InitContainers {
				if container.Image == "" {
					t.Error("expected container image to be set")
				}
				if container.Env == nil {
					t.Error("expected container env to be set")
				}
				if container.VolumeMounts == nil {
					t.Error("expected container VolumeMounts to be set")
				}
			}
		})
	}
}

func TestMariadbPodBuilderSidecarContainers(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-sidecarcontainers",
	}
	tests := []struct {
		name           string
		mariadb        *mariadbv1alpha1.MariaDB
		wantContainers int
	}{
		{
			name: "no sidecar containers",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					PodTemplate: mariadbv1alpha1.PodTemplate{
						SidecarContainers: nil,
					},
				},
			},
			wantContainers: 1,
		},
		{
			name: "sidecar containers",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					PodTemplate: mariadbv1alpha1.PodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Image: "busybox:latest",
							},
							{
								Image: "busybox:latest",
							},
						},
					},
				},
			},
			wantContainers: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podTpl, err := builder.mariadbPodTemplate(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB Pod template: %v", err)
			}

			if len(podTpl.Spec.Containers) != tt.wantContainers {
				t.Errorf("unexpected number of containers, got: %v, want: %v", len(podTpl.Spec.Containers), tt.wantContainers)
			}

			for _, container := range podTpl.Spec.Containers {
				if container.Image == "" {
					t.Error("expected container image to be set")
				}
				if container.Env == nil {
					t.Error("expected container env to be set")
				}
				if container.VolumeMounts == nil {
					t.Error("expected container VolumeMounts to be set")
				}
			}
		})
	}
}

func TestMaxscalePodBuilder(t *testing.T) {
	d, err := discovery.NewFakeDiscovery(false)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder := newTestBuilder(d)
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
	runAsGroup := ptr.Deref(sc.RunAsGroup, 0)
	fsGroup := ptr.Deref(sc.FSGroup, 0)

	if runAsUser != maxscaleUser {
		t.Errorf("expected to run as maxscale user, got user: %d", runAsUser)
	}
	if runAsGroup != maxscaleGroup {
		t.Errorf("expected to run as maxscale group, got group: %d", runAsGroup)
	}
	if fsGroup != maxscaleGroup {
		t.Errorf("expected to run as maxscale fsGroup, got fsGroup: %d", fsGroup)
	}
}

func TestMaxscaleEnterprisePodBuilder(t *testing.T) {
	d, err := discovery.NewFakeDiscovery(true)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder := newTestBuilder(d)
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
	runAsGroup := ptr.Deref(sc.RunAsGroup, 0)
	fsGroup := ptr.Deref(sc.FSGroup, 0)

	if runAsUser != maxscaleEnterpriseUser {
		t.Errorf("expected to run as maxscale user, got user: %d", runAsUser)
	}
	if runAsGroup != maxscaleEnterpriseGroup {
		t.Errorf("expected to run as maxscale group, got group: %d", runAsGroup)
	}
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

	mariadb.Spec.MyCnfConfigMapKeyRef = &mariadbv1alpha1.ConfigMapKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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
