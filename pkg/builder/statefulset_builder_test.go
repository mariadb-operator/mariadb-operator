package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMariadbImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-image-pull-secrets",
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
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
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
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildMariadbStatefulSet(tt.mariadb, client.ObjectKeyFromObject(tt.mariadb), nil)
			if err != nil {
				t.Fatalf("unexpected error building StatefulSet: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestMaxScaleImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		maxScale        *mariadbv1alpha1.MaxScale
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			maxScale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MaxScaleSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MaxScale",
			maxScale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "maxscale-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "maxscale-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildMaxscaleStatefulSet(tt.maxScale, client.ObjectKeyFromObject(tt.maxScale))
			if err != nil {
				t.Fatalf("unexpected error building StatefulSet: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestMariaDBStatefulSetMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	key := types.NamespacedName{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name           string
		mariadb        *mariadbv1alpha1.MariaDB
		podAnnotations map[string]string
		wantMeta       *mariadbv1alpha1.Metadata
		wantPodMeta    *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			podAnnotations: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
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
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			podAnnotations: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
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
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			podAnnotations: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
		},
		{
			name: "Pod annotations",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"k8s.mariadb.com/pod-meta": "pod-meta",
							},
						},
					},
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			podAnnotations: map[string]string{
				"k8s.mariadb.com/config": "config-hash",
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/pod-meta": "pod-meta",
					"k8s.mariadb.com/config":   "config-hash",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					PodTemplate: mariadbv1alpha1.PodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"k8s.mariadb.com/pod-meta": "pod-meta",
							},
						},
					},
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			podAnnotations: map[string]string{
				"k8s.mariadb.com/config": "config-hash",
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io":          "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io":        "mariadb",
					"k8s.mariadb.com/mariadb":  "mariadb-obj",
					"k8s.mariadb.com/galera":   "",
					"k8s.mariadb.com/pod-meta": "pod-meta",
					"k8s.mariadb.com/config":   "config-hash",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts, err := builder.BuildMariadbStatefulSet(tt.mariadb, key, tt.podAnnotations)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB StatefulSet: %v", err)
			}
			assertObjectMeta(t, &sts.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
			assertObjectMeta(t, &sts.Spec.Template.ObjectMeta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestMariaDBUpdateStrategy(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name               string
		mariadb            *mariadbv1alpha1.MariaDB
		wantUpdateStrategy *appsv1.StatefulSetUpdateStrategy
		wantErr            bool
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			wantUpdateStrategy: nil,
			wantErr:            true,
		},
		{
			name: "replicas first primary last",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLast,
					},
				},
			},
			wantUpdateStrategy: &appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			wantErr: false,
		},
		{
			name: "rolling update",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.RollingUpdateUpdateType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							MaxUnavailable: ptr.To(intstr.FromInt(1)),
						},
					},
				},
			},
			wantUpdateStrategy: &appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					MaxUnavailable: ptr.To(intstr.FromInt(1)),
				},
			},
			wantErr: false,
		},
		{
			name: "on delete",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.OnDeleteUpdateType,
					},
				},
			},
			wantUpdateStrategy: &appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stsStategy, err := mariadbUpdateStrategy(tt.mariadb)
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error building update strategy: %v", err)
			}
			if tt.wantErr && err == nil {
				t.Errorf("expected error building update strategy, got nil")
			}
			if !reflect.DeepEqual(tt.wantUpdateStrategy, stsStategy) {
				t.Errorf("expecting mariadbUpdateStrategy returned value to be:\n%v\ngot:\n%v\n", tt.wantUpdateStrategy, stsStategy)
			}
		})
	}
}

func TestMaxScaleStatefulSetMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "maxscale-obj",
	}
	key := types.NamespacedName{
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
				Labels:      map[string]string{},
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
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts, err := builder.BuildMaxscaleStatefulSet(tt.maxscale, key)
			if err != nil {
				t.Fatalf("unexpected error building MaxScale StatefulSet: %v", err)
			}
			assertObjectMeta(t, &sts.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestMariaDBVolumeClaimTemplates(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantVolumes []string
	}{
		{
			name: "ephemeral",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Ephemeral: ptr.To(true),
					},
				},
			},
			wantVolumes: []string{},
		},
		{
			name: "standalone",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("1Gi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
				},
			},
			wantVolumes: []string{StorageVolume},
		},
		{
			name: "replication",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("1Gi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			wantVolumes: []string{StorageVolume},
		},
		{
			name: "galera",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("1Gi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
									PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
										Resources: corev1.VolumeResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("1Gi"),
											},
										},
										AccessModes: []corev1.PersistentVolumeAccessMode{
											corev1.ReadWriteOnce,
										},
									},
								},
							},
						},
					},
				},
			},
			wantVolumes: []string{StorageVolume, galeraresources.GaleraConfigVolume},
		},
		{
			name: "galera reuse storage",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("1Gi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(true),
							},
						},
					},
				},
			},
			wantVolumes: []string{StorageVolume},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvcs := mariadbVolumeClaimTemplates(tt.mariadb)
			if len(pvcs) != len(tt.wantVolumes) {
				t.Errorf("unexpected number of PVCs, got: %v, want: %v", len(pvcs), len(tt.wantVolumes))
			}
			for _, wantVolume := range tt.wantVolumes {
				if !hasVolume(pvcs, wantVolume) {
					t.Errorf("expecting Volume \"%s\", but it was not found", wantVolume)
				}
			}
		})
	}
}

func hasVolume(pvcs []corev1.PersistentVolumeClaim, volumeName string) bool {
	for _, p := range pvcs {
		if p.Name == volumeName {
			return true
		}
	}
	return false
}
