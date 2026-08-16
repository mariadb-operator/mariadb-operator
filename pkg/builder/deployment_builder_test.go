package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ExporterImagePullSecrets", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-metrics-image-pull-secrets",
		Namespace: "test",
	}

	DescribeTable(
		"should build the expected ImagePullSecrets",
		func(mariadb *mariadbv1alpha1.MariaDB, wantPullSecrets []corev1.LocalObjectReference) {
			job, err := builder.BuildExporterDeployment(mariadb, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry(
			"No Secrets",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			nil,
		),
		Entry(
			"Secrets in MariaDB",
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
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		),
		Entry(
			"Secrets in Exporter",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
								{
									Name: "exporter-registry",
								},
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "exporter-registry",
				},
			},
		),
		Entry(
			"Secrets in MariaDB and Exporter",
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
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
								{
									Name: "exporter-registry",
								},
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
				{
					Name: "exporter-registry",
				},
			},
		),
	)
})

var _ = Describe("ExporterMaxScaleImagePullSecrets", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-metrics-image-pull-secrets",
		Namespace: "test",
	}

	DescribeTable(
		"should build the expected ImagePullSecrets",
		func(maxscale *mariadbv1alpha1.MaxScale, wantPullSecrets []corev1.LocalObjectReference) {
			job, err := builder.BuildMaxScaleExporterDeployment(maxscale, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(Equal(wantPullSecrets))
		},
		Entry(
			"No Secrets",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
					},
				},
			},
			nil,
		),
		Entry(
			"Secrets in MaxScale",
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
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "maxscale-registry",
				},
			},
		),
		Entry(
			"Secrets in MaxScale",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
								{
									Name: "exporter-registry",
								},
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "exporter-registry",
				},
			},
		),
		Entry(
			"Secrets in MariaDB and Exporter",
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
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{
								{
									Name: "exporter-registry",
								},
							},
						},
					},
				},
			},
			[]corev1.LocalObjectReference{
				{
					Name: "maxscale-registry",
				},
				{
					Name: "exporter-registry",
				},
			},
		),
	)
})

var _ = Describe("ExporterResources", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-metrics-resources",
		Namespace: "test",
	}

	DescribeTable(
		"should build the expected Resources",
		func(mariadb *mariadbv1alpha1.MariaDB, wantResources corev1.ResourceRequirements) {
			job, err := builder.BuildExporterDeployment(mariadb, nil)
			Expect(err).NotTo(HaveOccurred())
			resources := job.Spec.Template.Spec.Containers[0].Resources
			Expect(resources).To(Equal(wantResources))
		},
		Entry(
			"No Resources",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			corev1.ResourceRequirements{},
		),
		Entry(
			"Resources",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							Resources: &mariadbv1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									"memory": resource.MustParse("100Mi"),
								},
							},
						},
					},
				},
			},
			corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
				Limits: corev1.ResourceList{
					"memory": resource.MustParse("100Mi"),
				},
			},
		),
	)
})

var _ = Describe("ExporterSecurityContext", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-metrics-security-context",
		Namespace: "test",
	}

	DescribeTable(
		"should build the expected SecurityContext",
		func(mariadb *mariadbv1alpha1.MariaDB, wantSecurityContext *corev1.SecurityContext) {
			job, err := builder.BuildExporterDeployment(mariadb, nil)
			Expect(err).NotTo(HaveOccurred())
			securityContext := job.Spec.Template.Spec.Containers[0].SecurityContext
			Expect(securityContext).To(Equal(wantSecurityContext))
		},
		Entry(
			"No SecurityContext",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			nil,
		),
		Entry(
			"SecurityContext",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							SecurityContext: &mariadbv1alpha1.SecurityContext{
								RunAsUser: ptr.To(int64(666)),
							},
						},
					},
				},
			},
			&corev1.SecurityContext{
				RunAsUser: ptr.To(int64(666)),
			},
		),
	)
})

var _ = Describe("ExporterPodSecurityContext", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-metrics-pod-security-context",
		Namespace: "test",
	}

	DescribeTable(
		"should build the expected PodSecurityContext",
		func(mariadb *mariadbv1alpha1.MariaDB, wantSecurityContext *corev1.PodSecurityContext) {
			job, err := builder.BuildExporterDeployment(mariadb, nil)
			Expect(err).NotTo(HaveOccurred())
			securityContext := job.Spec.Template.Spec.SecurityContext
			Expect(securityContext).To(Equal(wantSecurityContext))
		},
		Entry(
			"No PodSecurityContext",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			nil,
		),
		Entry(
			"PodSecurityContext",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							PodSecurityContext: &mariadbv1alpha1.PodSecurityContext{
								RunAsUser: ptr.To(int64(666)),
							},
						},
					},
				},
			},
			&corev1.PodSecurityContext{
				RunAsUser: ptr.To(int64(666)),
			},
		),
	)
})

var _ = Describe("ExporterDeploymentMeta", func() {
	builder := newDefaultTestBuilder()
	mdbObjMeta := metav1.ObjectMeta{
		Name: "test",
	}

	DescribeTable(
		"should build the expected Deployment metadata",
		func(mariadb *mariadbv1alpha1.MariaDB, podAnnotations map[string]string,
			wantDeployMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			deploy, err := builder.BuildExporterDeployment(mariadb, podAnnotations)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&deploy.ObjectMeta, wantDeployMeta.Labels, wantDeployMeta.Annotations)
			assertObjectMeta(&deploy.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry(
			"no meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
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
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
				},
				Annotations: map[string]string{},
			},
		),
		Entry(
			"inherit meta",
			&mariadbv1alpha1.MariaDB{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry(
			"pod meta",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
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
			map[string]string{
				metadata.ConfigAnnotation: "config-hash",
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "pod",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "pod",
					metadata.ConfigAnnotation: "config-hash",
				},
			},
		),
		Entry(
			"all",
			&mariadbv1alpha1.MariaDB{
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
			map[string]string{
				metadata.ConfigAnnotation: "config-hash",
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "pod",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "pod",
					metadata.ConfigAnnotation: "config-hash",
				},
			},
		),
	)
})

var _ = Describe("ExporterMaxScaleDeploymentMeta", func() {
	builder := newDefaultTestBuilder()
	mxsObjMeta := metav1.ObjectMeta{
		Name: "test",
	}

	DescribeTable(
		"should build the expected Deployment metadata",
		func(maxscale *mariadbv1alpha1.MaxScale, podAnnotations map[string]string,
			wantDeployMeta *mariadbv1alpha1.Metadata, wantPodMeta *mariadbv1alpha1.Metadata) {
			deploy, err := builder.BuildMaxScaleExporterDeployment(maxscale, podAnnotations)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&deploy.ObjectMeta, wantDeployMeta.Labels, wantDeployMeta.Annotations)
			assertObjectMeta(&deploy.Spec.Template.ObjectMeta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry(
			"no meta",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: mxsObjMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
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
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
				},
				Annotations: map[string]string{},
			},
		),
		Entry(
			"inherit meta",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: mxsObjMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
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
			nil,
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry(
			"pod meta",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: mxsObjMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
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
			map[string]string{
				metadata.ConfigAnnotation: "config-hash",
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "pod",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "pod",
					metadata.ConfigAnnotation: "config-hash",
				},
			},
		),
		Entry(
			"all",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: mxsObjMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					Metrics: &mariadbv1alpha1.MaxScaleMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
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
			map[string]string{
				metadata.ConfigAnnotation: "config-hash",
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/instance": "test-metrics",
					"app.kubernetes.io/name":     "exporter",
					"database.myorg.io":          "pod",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "pod",
					metadata.ConfigAnnotation: "config-hash",
				},
			},
		),
	)
})

var _ = Describe("ExporterVolumes", func() {
	builder := newDefaultTestBuilder()

	DescribeTable(
		"should build the expected Volumes",
		func(mariadb *mariadbv1alpha1.MariaDB, wantVolumeNames []string) {
			deploy, err := builder.BuildExporterDeployment(mariadb, nil)
			Expect(err).NotTo(HaveOccurred())

			volumes := deploy.Spec.Template.Spec.Volumes
			volumeMounts := deploy.Spec.Template.Spec.Containers[0].VolumeMounts

			volumeIndex := datastructures.NewIndex(volumes, func(v corev1.Volume) string {
				return v.Name
			})
			volumeMountIndex := datastructures.NewIndex(volumeMounts, func(vm corev1.VolumeMount) string {
				return vm.Name
			})

			Expect(datastructures.AllExists(volumeIndex, wantVolumeNames...)).To(BeTrue())
			Expect(datastructures.AllExists(volumeMountIndex, wantVolumeNames...)).To(BeTrue())
		},
		Entry(
			"empty",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			[]string{
				deployConfigVolume,
			},
		),
		Entry(
			"TLS",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			[]string{
				deployConfigVolume,
				builderpki.PKIVolume,
			},
		),
	)
})

var _ = Describe("ExporterArgs", func() {
	builder := newDefaultTestBuilder()
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-metrics-resources",
		Namespace: "test",
	}

	DescribeTable(
		"should build the expected Args",
		func(mariadb *mariadbv1alpha1.MariaDB, wantArgs []string) {
			deploy, err := builder.BuildExporterDeployment(mariadb, nil)
			Expect(err).NotTo(HaveOccurred())

			args := deploy.Spec.Template.Spec.Containers[0].Args
			Expect(args).To(Equal(wantArgs))
		},
		Entry(
			"Without args",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			[]string{
				"--config.my-cnf=/etc/config/exporter.cnf",
			},
		),
		Entry(
			"With args",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
						Exporter: mariadbv1alpha1.Exporter{
							Args: []string{
								"--no-collect.auto_increment.columns",
								"--collect.sys.user_summary",
							},
						},
					},
				},
			},
			[]string{
				"--config.my-cnf=/etc/config/exporter.cnf",
				"--no-collect.auto_increment.columns",
				"--collect.sys.user_summary",
			},
		),
	)
})
