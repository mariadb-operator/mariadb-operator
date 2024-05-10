package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Base types", func() {
	Context("When creating a RestoreSource object", func() {
		DescribeTable(
			"Should default",
			func(
				rs *RestoreSource,
				backup *Backup,
				wantRestoreSource *RestoreSource,
				wantDefaulted bool,
				wantBackupDefaulErr bool,
			) {
				rs.SetDefaults()
				if backup != nil {
					err := rs.SetDefaultsWithBackup(backup)
					if wantBackupDefaulErr {
						Expect(err).To(HaveOccurred())
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				}
				Expect(rs).To(BeEquivalentTo(wantRestoreSource))
				Expect(rs.IsDefaulted()).To(Equal(wantDefaulted))
			},
			Entry(
				"Empty",
				&RestoreSource{},
				&Backup{},
				&RestoreSource{},
				false,
				true,
			),
			Entry(
				"S3",
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
				},
				nil,
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"Volume",
				&RestoreSource{
					Volume: &corev1.VolumeSource{
						NFS: &corev1.NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
				},
				nil,
				&RestoreSource{
					Volume: &corev1.VolumeSource{
						NFS: &corev1.NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
				},
				true,
				false,
			),
			Entry(
				"S3 priority over Volume",
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &corev1.VolumeSource{
						NFS: &corev1.NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
				},
				nil,
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"Backup S3",
				&RestoreSource{},
				&Backup{
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
					},
				},
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"Backup Volume",
				&RestoreSource{},
				&Backup{
					Spec: BackupSpec{
						Storage: BackupStorage{
							Volume: &corev1.VolumeSource{
								NFS: &corev1.NFSVolumeSource{
									Server: "test",
									Path:   "test",
								},
							},
						},
					},
				},
				&RestoreSource{
					Volume: &corev1.VolumeSource{
						NFS: &corev1.NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
				},
				true,
				false,
			),
			Entry(
				"Backup priority over S3",
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
				},
				&Backup{
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test-backup",
								Endpoint: "test-backup",
							},
						},
					},
				},
				&RestoreSource{
					S3: &S3{
						Bucket:   "test-backup",
						Endpoint: "test-backup",
					},
					Volume: &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"Backup priority over Volume",
				&RestoreSource{
					Volume: &corev1.VolumeSource{
						NFS: &corev1.NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
				},
				&Backup{
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test-backup",
								Endpoint: "test-backup",
							},
						},
					},
				},
				&RestoreSource{
					S3: &S3{
						Bucket:   "test-backup",
						Endpoint: "test-backup",
					},
					Volume: &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
		)
	})

	Context("When creating an Affinity object", func() {
		DescribeTable(
			"Should default",
			func(
				affinity *AffinityConfig,
				antiAffinityInstances []string,
				wantAffinity *AffinityConfig,
			) {
				affinity.SetDefaults(antiAffinityInstances...)
				Expect(affinity).To(BeEquivalentTo(wantAffinity))
			},
			Entry(
				"Empty",
				&AffinityConfig{},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{},
			),
			Entry(
				"Anti-affinity disabled",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(false),
				},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(false),
				},
			),
			Entry(
				"Already defaulted",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb", "maxscale"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb", "maxscale"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			),
			Entry(
				"No instances",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
				nil,
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
			),
			Entry(
				"Single instance",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
				[]string{"mariadb"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			),
			Entry(
				"Multiple instances",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
				},
				[]string{"mariadb", "maxscale"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/instance",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"mariadb", "maxscale"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			),
		)
	})

	Context("When merging multiple Metadata instances", func() {
		DescribeTable(
			"Should succeed",
			func(
				metas []*Metadata,
				wantMeta *Metadata,
			) {
				gotMeta := MergeMetadata(metas...)
				Expect(wantMeta).To(BeEquivalentTo(gotMeta))
			},
			Entry(
				"empty",
				[]*Metadata{},
				&Metadata{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			),
			Entry(
				"single",
				[]*Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
				&Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
			),
			Entry(
				"multiple",
				[]*Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
					},
				},
				&Metadata{
					Labels: map[string]string{
						"database.myorg.io":       "mariadb",
						"sidecar.istio.io/inject": "false",
					},
					Annotations: map[string]string{
						"database.myorg.io":       "mariadb",
						"sidecar.istio.io/inject": "false",
					},
				},
			),
			Entry(
				"override",
				[]*Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					{
						Labels: map[string]string{
							"database.myorg.io": "mydb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mydb",
						},
					},
				},
				&Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mydb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mydb",
					},
				},
			),
		)
	})
})
