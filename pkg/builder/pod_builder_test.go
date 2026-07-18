package builder

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariadbPodMeta", func() {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	DescribeTable("mariadbPodTemplate meta",
		func(mariadb *mariadbv1alpha1.MariaDB, opts []mariadbPodOpt, wantMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.mariadbPodTemplate(mariadb, opts...)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&podTpl.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
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
				},
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
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
				},
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("extra meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			[]mariadbPodOpt{
				withMeta(&mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				}),
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("inherit and Pod meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("extra override Pod meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			[]mariadbPodOpt{
				withMeta(&mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
				}),
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "true",
				},
				Annotations: map[string]string{},
			},
		),
		Entry("Pod override inherit meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("without selector labels",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			[]mariadbPodOpt{
				withMariadbSelectorLabels(false),
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{},
			},
		),
		Entry("without HA annotations",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			[]mariadbPodOpt{
				withHAAnnotations(false),
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.MariaDB{
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
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						PodMetadata: &mariadbv1alpha1.Metadata{
							Labels: map[string]string{
								"sidecar.istio.io/inject": "false",
							},
						},
					},
				},
			},
			[]mariadbPodOpt{
				withMeta(&mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				}),
			},
			&mariadbv1alpha1.Metadata{
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
		),
	)
})

var _ = Describe("MaxScalePodMeta", func() {
	objMeta := metav1.ObjectMeta{
		Name: "maxscale-obj",
	}
	DescribeTable("maxscalePodTemplate meta",
		func(maxscale *mariadbv1alpha1.MaxScale, annotations map[string]string, wantMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.maxscalePodTemplate(maxscale, annotations)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&podTpl.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("empty",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
			},
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
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
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod meta",
			&mariadbv1alpha1.MaxScale{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("annotations",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
			},
			map[string]string{
				metadata.TLSServerCertAnnotation: "cert",
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
				},
				Annotations: map[string]string{
					metadata.TLSServerCertAnnotation: "cert",
				},
			},
		),
		Entry("inherit and Pod meta",
			&mariadbv1alpha1.MaxScale{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("Pod override inherit meta",
			&mariadbv1alpha1.MaxScale{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("all",
			&mariadbv1alpha1.MaxScale{
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
			map[string]string{
				metadata.TLSServerCertAnnotation: "cert",
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "maxscale",
					"app.kubernetes.io/instance": "maxscale-obj",
					"sidecar.istio.io/inject":    "false",
					"k8s.mariadb.com":            "test",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com":                "test",
					"database.myorg.io":              "mariadb",
					metadata.TLSServerCertAnnotation: "cert",
				},
			},
		),
	)
})

var _ = Describe("MariadbPodBuilder", func() {
	It("should build the MariaDB Pod template", func() {
		builder := newDefaultTestBuilder()
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
		Expect(err).NotTo(HaveOccurred())

		Expect(reflect.ValueOf(podTpl.Spec.Containers[0].Resources).IsZero()).To(BeFalse())

		Expect(podTpl.Spec.SecurityContext).NotTo(BeNil())
		sc := ptr.Deref(podTpl.Spec.SecurityContext, corev1.PodSecurityContext{})
		runAsUser := ptr.Deref(sc.RunAsUser, 0)
		Expect(runAsUser).To(Equal(mysqlUser))
		runAsGroup := ptr.Deref(sc.RunAsGroup, 0)
		Expect(runAsGroup).To(Equal(mysqlGroup))
		fsGroup := ptr.Deref(sc.FSGroup, 0)
		Expect(fsGroup).To(Equal(mysqlGroup))
	})
})

var _ = Describe("MariadbPodBuilderResources", func() {
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-resources",
	}
	DescribeTable("mariadbPodTemplate resources",
		func(mariadb *mariadbv1alpha1.MariaDB, opts []mariadbPodOpt, wantResources corev1.ResourceRequirements) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.mariadbPodTemplate(mariadb, opts...)
			Expect(err).NotTo(HaveOccurred())
			Expect(podTpl.Spec.Containers).To(HaveLen(1))
			resources := podTpl.Spec.Containers[0].Resources
			Expect(resources).To(Equal(wantResources))
		},
		Entry("no resources",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			nil,
			corev1.ResourceRequirements{},
		),
		Entry("mariadb resources",
			&mariadbv1alpha1.MariaDB{
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
			nil,
			corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("300m"),
				},
			},
		),
		Entry("opt resources",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			[]mariadbPodOpt{
				withResources(&corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu": resource.MustParse("100m"),
					},
				}),
				withMariadbResources(false),
			},
			corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		),
		Entry("mariadb and opt resources",
			&mariadbv1alpha1.MariaDB{
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
			[]mariadbPodOpt{
				withResources(&corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu": resource.MustParse("100m"),
					},
				}),
				withMariadbResources(true),
			},
			corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("100m"),
				},
			},
		),
	)
})

var _ = Describe("MariadbPodBuilderServiceAccount", func() {
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-serviceaccount",
	}
	DescribeTable("mariadbPodTemplate serviceAccount",
		func(mariadb *mariadbv1alpha1.MariaDB, opts []mariadbPodOpt, wantServiceAccount bool) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.mariadbPodTemplate(mariadb, opts...)
			Expect(err).NotTo(HaveOccurred())
			Expect(podTpl.Spec.Containers).NotTo(BeEmpty())

			container := podTpl.Spec.Containers[0]
			scName := podTpl.Spec.ServiceAccountName
			scVol := datastructures.Find(podTpl.Spec.Volumes, func(vol corev1.Volume) bool {
				return vol.Name == ServiceAccountVolume
			})
			scVolMount := datastructures.Find(container.VolumeMounts, func(volMount corev1.VolumeMount) bool {
				return volMount.Name == ServiceAccountVolume
			})

			if wantServiceAccount {
				Expect(scName).To(Equal(objMeta.Name))
				Expect(scVol).NotTo(BeNil())
				Expect(scVolMount).NotTo(BeNil())
			} else {
				Expect(scName).To(Equal(""))
				Expect(scVol).To(BeNil())
				Expect(scVolMount).To(BeNil())
			}
		},
		Entry("serviceaccount",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			nil,
			true,
		),
		Entry("no serviceaccount",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			[]mariadbPodOpt{
				withServiceAccount(false),
			},
			false,
		),
	)
})

var _ = Describe("MariadbPodBuilderAffinity", func() {
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-affinity",
	}
	DescribeTable("mariadbPodTemplate affinity",
		func(mariadb *mariadbv1alpha1.MariaDB, opts []mariadbPodOpt,
			wantAffinity, wantTopologySpreadConstraints, wantNodeAffinity bool) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.mariadbPodTemplate(mariadb, opts...)
			Expect(err).NotTo(HaveOccurred())

			if wantAffinity {
				Expect(podTpl.Spec.Affinity).NotTo(BeNil())
			} else {
				Expect(podTpl.Spec.Affinity).To(BeNil())
			}

			if wantTopologySpreadConstraints {
				Expect(podTpl.Spec.TopologySpreadConstraints).NotTo(BeNil())
			} else {
				Expect(podTpl.Spec.TopologySpreadConstraints).To(BeNil())
			}

			if wantNodeAffinity {
				Expect(podTpl.Spec.Affinity.NodeAffinity).NotTo(BeNil())
			}
			if !wantNodeAffinity && podTpl.Spec.Affinity != nil {
				Expect(podTpl.Spec.Affinity.NodeAffinity).To(BeNil())
			}
		},
		Entry("no affinity",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			nil,
			false,
			false,
			false,
		),
		Entry("mariadb affinity",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							AntiAffinityEnabled: ptr.To(true),
						},
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			nil,
			true,
			false,
			false,
		),
		Entry("mariadb topologyspreadconstraints",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			nil,
			false,
			true,
			false,
		),
		Entry("opt affinity",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			},
			[]mariadbPodOpt{
				withAffinity(&corev1.Affinity{}),
				withAffinityEnabled(true),
			},
			true,
			false,
			false,
		),
		Entry("mariadb and opt affinity",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			[]mariadbPodOpt{
				withAffinity(&corev1.Affinity{}),
				withAffinityEnabled(true),
			},
			true,
			true,
			false,
		),
		Entry("disable affinity",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			[]mariadbPodOpt{
				withAffinity(&corev1.Affinity{}),
				withAffinityEnabled(false),
			},
			false,
			false,
			false,
		),
		Entry("mariadb with node affinity",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			nil,
			true,
			false,
			true,
		),
	)
})

var _ = Describe("MariadbPodLifecycleOnlyFirst", func() {
	It("should only set lifecycle on the first container", func() {
		builder := newDefaultTestBuilder()

		mariadb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-mariadb-lifecycle",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
					Lifecycle: &mariadbv1alpha1.Lifecycle{
						PostStart: &mariadbv1alpha1.LifecycleHandler{
							Exec: &mariadbv1alpha1.ExecAction{
								Command: []string{"echo", "hello"},
							},
						},
					},
				},
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					SidecarContainers: []mariadbv1alpha1.Container{
						{
							Image: "busybox",
							Command: []string{
								"sh",
								"-c",
								"sleep 1",
							},
						},
					},
				},
			},
		}

		podTpl, err := builder.mariadbPodTemplate(mariadb)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(podTpl.Spec.Containers)).To(BeNumerically(">=", 2))

		Expect(podTpl.Spec.Containers[0].Lifecycle).NotTo(BeNil())
		Expect(podTpl.Spec.Containers[1].Lifecycle).To(BeNil())
	})
})

var _ = Describe("MaxScalePodLifecycleSingle", func() {
	It("should set lifecycle on the maxscale container", func() {
		builder := newDefaultTestBuilder()

		mxs := &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-maxscale-lifecycle",
			},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
					Lifecycle: &mariadbv1alpha1.Lifecycle{
						PostStart: &mariadbv1alpha1.LifecycleHandler{
							Exec: &mariadbv1alpha1.ExecAction{
								Command: []string{"echo", "hello"},
							},
						},
					},
				},
			},
		}

		podTpl, err := builder.maxscalePodTemplate(mxs, nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(podTpl.Spec.Containers).To(HaveLen(1))

		Expect(podTpl.Spec.Containers[0].Lifecycle).NotTo(BeNil())
	})
})

var _ = Describe("MariadbPodBuilderInitContainers", func() {
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-initcontainers",
	}
	DescribeTable("mariadbPodTemplate initContainers",
		func(mariadb *mariadbv1alpha1.MariaDB, wantInitContainers int) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.mariadbPodTemplate(mariadb)
			Expect(err).NotTo(HaveOccurred())

			Expect(podTpl.Spec.InitContainers).To(HaveLen(wantInitContainers))

			for _, container := range podTpl.Spec.InitContainers {
				Expect(container.Image).NotTo(Equal(""))
				Expect(container.Env).NotTo(BeNil())
				Expect(container.VolumeMounts).NotTo(BeNil())
			}
		},
		Entry("no init containers",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						InitContainers: nil,
					},
				},
			},
			0,
		),
		Entry("init containers",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			2,
		),
	)
})

var _ = Describe("MariadbPodBuilderSidecarContainers", func() {
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-builder-sidecarcontainers",
	}
	DescribeTable("mariadbPodTemplate sidecarContainers",
		func(mariadb *mariadbv1alpha1.MariaDB, wantContainers int) {
			builder := newDefaultTestBuilder()
			podTpl, err := builder.mariadbPodTemplate(mariadb)
			Expect(err).NotTo(HaveOccurred())

			Expect(podTpl.Spec.Containers).To(HaveLen(wantContainers))

			for _, container := range podTpl.Spec.Containers {
				Expect(container.Image).NotTo(Equal(""))
				Expect(container.Env).NotTo(BeNil())
				Expect(container.VolumeMounts).NotTo(BeNil())
			}
		},
		Entry("no sidecar containers",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						SidecarContainers: nil,
					},
				},
			},
			1,
		),
		Entry("sidecar containers",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:11.4.3",
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
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
			3,
		),
	)
})

var _ = Describe("MaxscalePodBuilder", func() {
	It("should build the MaxScale Pod template", func() {
		d, err := discovery.NewFakeDiscovery()
		Expect(err).NotTo(HaveOccurred())
		builder := newTestBuilder(d)
		mxs := &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-maxscale-builder",
			},
		}

		podTpl, err := builder.maxscalePodTemplate(mxs, nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(podTpl.Spec.SecurityContext).NotTo(BeNil())
		sc := ptr.Deref(podTpl.Spec.SecurityContext, corev1.PodSecurityContext{})
		runAsUser := ptr.Deref(sc.RunAsUser, 0)
		runAsGroup := ptr.Deref(sc.RunAsGroup, 0)
		fsGroup := ptr.Deref(sc.FSGroup, 0)

		Expect(runAsUser).To(Equal(maxscaleUser))
		Expect(runAsGroup).To(Equal(maxscaleGroup))
		Expect(fsGroup).To(Equal(maxscaleGroup))
	})
})

var _ = Describe("MariadbConfigVolume", func() {
	It("should build the MariaDB config volume", func() {
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
		Expect(volume.Projected).NotTo(BeNil())
		expectedSources := 1
		Expect(volume.Projected.Sources).To(HaveLen(expectedSources))
		expectedKey := "0-default.cnf"
		Expect(volume.Projected.Sources[0].ConfigMap.Items[0].Key).To(Equal(expectedKey))

		mariadb.Spec.MyCnfConfigMapKeyRef = &mariadbv1alpha1.ConfigMapKeySelector{
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: "test",
			},
			Key: "my.cnf",
		}

		volume = mariadbConfigVolume(mariadb)
		Expect(volume.Projected).NotTo(BeNil())
		expectedSources = 2
		Expect(volume.Projected.Sources).To(HaveLen(expectedSources))
		expectedKey = "0-default.cnf"
		Expect(volume.Projected.Sources[0].ConfigMap.Items[0].Key).To(Equal(expectedKey))
		expectedKey = "my.cnf"
		Expect(volume.Projected.Sources[1].ConfigMap.Items[0].Key).To(Equal(expectedKey))
	})
})

var _ = Describe("MariadbTerminationGracePeriodSeconds", func() {
	objMeta := metav1.ObjectMeta{
		Name: "test-mariadb-termination-grace",
	}
	DescribeTable("mariadbPodTemplate terminationGracePeriodSeconds",
		func(tgs *int64) {
			builder := newDefaultTestBuilder()
			mariadb := &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
						TerminationGracePeriodSeconds: tgs,
					},
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			}

			podTpl, err := builder.mariadbPodTemplate(mariadb)
			Expect(err).NotTo(HaveOccurred())
			Expect(podTpl.Spec.TerminationGracePeriodSeconds).To(Equal(tgs))
		},
		Entry("unset", nil),
		Entry("set", ptr.To(int64(5))),
	)
})
