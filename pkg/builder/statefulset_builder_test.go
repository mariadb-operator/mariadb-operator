package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariadbImagePullSecrets", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-image-pull-secrets",
		Namespace: "test",
	}

	DescribeTable("should build the expected ImagePullSecrets",
		func(mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			job, err := builder.BuildMariadbStatefulSet(mariadb, client.ObjectKeyFromObject(mariadb), nil, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			nil,
		),
		Entry("Secrets in MariaDB",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
	)
})

var _ = Describe("MaxScaleImagePullSecrets", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-image-pull-secrets",
		Namespace: "test",
	}

	DescribeTable("should build the expected ImagePullSecrets",
		func(maxScale *mariadbv1alpha1.MaxScale, wantPullSecrets []corev1.LocalObjectReference) {
			job, err := builder.BuildMaxscaleStatefulSet(maxScale, client.ObjectKeyFromObject(maxScale), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry("No Secrets",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MaxScaleSpec{},
			},
			nil,
		),
		Entry("Secrets in MaxScale",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					MaxScalePodTemplate: mariadbv1alpha1.MaxScalePodTemplate{
						ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
							{
								Name: "maxscale-registry",
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "maxscale-registry",
				},
			},
		),
	)
})

var _ = Describe("MariaDBStatefulSetMeta", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	key := types.NamespacedName{
		Name: "mariadb-obj",
	}

	DescribeTable("should build the expected meta",
		func(mariadb *mariadbv1alpha1.MariaDB, podAnnotations map[string]string,
			wantMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			sts, err := builder.BuildMariadbStatefulSet(mariadb, key, podAnnotations, nil)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&sts.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
			assertObjectMeta(&sts.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit meta",
			&mariadbv1alpha1.MariaDB{
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
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject":    "false",
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("HA",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
		),
		Entry("Pod annotations",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"k8s.mariadb.com/pod-meta": "pod-meta",
							},
						},
					},
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			map[string]string{
				"k8s.mariadb.com/config": "config-hash",
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "mariadb-obj",
					"app.kubernetes.io/name":     "mariadb",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/pod-meta": "pod-meta",
					"k8s.mariadb.com/config":   "config-hash",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"k8s.mariadb.com/pod-meta": "pod-meta",
							},
						},
					},
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			map[string]string{
				"k8s.mariadb.com/config": "config-hash",
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
			&mariadbv1alpha1.Metadata{
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
		),
	)
})

var _ = Describe("MariaDBUpdateStrategy", func() {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}

	DescribeTable("should build the expected update strategy",
		func(mariadb *mariadbv1alpha1.MariaDB, wantUpdateStrategy *appsv1.StatefulSetUpdateStrategy, wantErr bool) {
			stsStrategy, err := mariadbUpdateStrategy(mariadb)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(stsStrategy).To(Equal(wantUpdateStrategy))
		},
		Entry("empty",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			nil,
			true,
		),
		Entry("replicas first primary last",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
					},
				},
			},
			&appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			false,
		),
		Entry("rolling update",
			&mariadbv1alpha1.MariaDB{
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
			&appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					MaxUnavailable: ptr.To(intstr.FromInt(1)),
				},
			},
			false,
		),
		Entry("on delete",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.OnDeleteUpdateType,
					},
				},
			},
			&appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
			false,
		),
		Entry("never",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.NeverUpdateType,
					},
				},
			},
			&appsv1.StatefulSetUpdateStrategy{},
			false,
		),
	)
})

var _ = Describe("MariaDBPVCRetentionPolicy", func() {
	DescribeTable("should set the expected PVC retention policy",
		func(gitVersion string, wantSet bool) {
			discoveryClient, err := discovery.NewFakeDiscoveryWithServerVersion(&version.Info{
				GitVersion: gitVersion,
			})
			Expect(err).NotTo(HaveOccurred())
			builder := newTestBuilder(discoveryClient)
			mariadb := &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "mariadb-obj"},
				Spec: mariadbv1alpha1.MariaDBSpec{
					UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
						Type: mariadbv1alpha1.RollingUpdateUpdateType,
					},
					Storage: mariadbv1alpha1.Storage{
						PVCRetentionPolicy: &mariadbv1alpha1.StatefulSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: mariadbv1alpha1.PersistentVolumeClaimRetentionPolicyDelete,
							WhenScaled:  mariadbv1alpha1.PersistentVolumeClaimRetentionPolicyRetain,
						},
					},
				},
			}

			sts, err := builder.BuildMariadbStatefulSet(mariadb, client.ObjectKeyFromObject(mariadb), nil, nil)
			Expect(err).NotTo(HaveOccurred())

			if wantSet {
				Expect(sts.Spec.PersistentVolumeClaimRetentionPolicy).NotTo(BeNil())
				Expect(sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted).
					To(Equal(appsv1.DeletePersistentVolumeClaimRetentionPolicyType))
				Expect(sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled).
					To(Equal(appsv1.RetainPersistentVolumeClaimRetentionPolicyType))
			} else {
				Expect(sts.Spec.PersistentVolumeClaimRetentionPolicy).To(BeNil())
			}
		},
		Entry("supported kubernetes version", "v1.32.0", true),
		Entry("unsupported kubernetes version", "v1.31.0", false),
	)
})

var _ = Describe("MaxScaleStatefulSetMeta", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name: "maxscale-obj",
	}
	key := types.NamespacedName{
		Name: "maxscale-obj",
	}

	DescribeTable("should build the expected meta",
		func(maxscale *mariadbv1alpha1.MaxScale, podAnnotations map[string]string,
			wantMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			sts, err := builder.BuildMaxscaleStatefulSet(maxscale, key, podAnnotations)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&sts.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
			assertObjectMeta(&sts.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "maxscale-obj",
					"app.kubernetes.io/name":     "maxscale",
				},
				Annotations: map[string]string{},
			},
		),
		Entry("inherit meta",
			&mariadbv1alpha1.MaxScale{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "maxscale-obj",
					"app.kubernetes.io/name":     "maxscale",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod annotations",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
			},
			map[string]string{
				metadata.TLSServerCertAnnotation: "cert",
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "maxscale-obj",
					"app.kubernetes.io/name":     "maxscale",
				},
				Annotations: map[string]string{
					metadata.TLSServerCertAnnotation: "cert",
				},
			},
		),
	)
})

var _ = Describe("MariaDBVolumeClaimTemplates", func() {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}

	DescribeTable("should build the expected volume claim templates",
		func(mariadb *mariadbv1alpha1.MariaDB, wantVolumes []string) {
			pvcs := mariadbVolumeClaimTemplates(mariadb)
			Expect(pvcs).To(HaveLen(len(wantVolumes)))
			for _, wantVolume := range wantVolumes {
				Expect(hasVolume(pvcs, wantVolume)).To(BeTrue())
			}
		},
		Entry("ephemeral",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Ephemeral: ptr.To(true),
					},
				},
			},
			[]string{},
		),
		Entry("standalone",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
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
			[]string{StorageVolume},
		),
		Entry("replication",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
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
			[]string{StorageVolume},
		),
		Entry("galera",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
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
									PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
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
			[]string{StorageVolume, galeraresources.GaleraConfigVolume},
		),
		Entry("galera reuse storage",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("1Gi")),
						VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
							PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
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
			[]string{StorageVolume},
		),
	)
})

func hasVolume(pvcs []corev1.PersistentVolumeClaim, volumeName string) bool {
	for _, p := range pvcs {
		if p.Name == volumeName {
			return true
		}
	}
	return false
}
