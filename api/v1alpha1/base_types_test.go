package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Base types", func() {
	Context("When creating a RestoreSource object", func() {
		restore := Restore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "restore-default",
				Namespace: testNamespace,
			},
		}
		DescribeTable(
			"Should default",
			func(
				rs *RestoreSource,
				backup *Backup,
				wantRestoreSource *RestoreSource,
				wantDefaulted bool,
				wantBackupDefaulErr bool,
			) {
				rs.SetDefaults(&restore)
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
					Volume: &StorageVolumeSource{
						EmptyDir: &EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"S3 with staging",
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
							StorageClassName: ptr.To("my-sc"),
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
				nil,
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &StorageVolumeSource{
						PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
							ClaimName: restore.StagingPVCKey().Name,
						},
					},
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
							StorageClassName: ptr.To("my-sc"),
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
				true,
				false,
			),
			Entry(
				"Volume",
				&RestoreSource{
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
				},
				nil,
				&RestoreSource{
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
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
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
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
					Volume: &StorageVolumeSource{
						EmptyDir: &EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"S3 with staging priority over Volume",
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
							Server: "test",
							Path:   "test",
						},
					},
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
							StorageClassName: ptr.To("my-sc"),
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
				nil,
				&RestoreSource{
					S3: &S3{
						Bucket:   "test",
						Endpoint: "test",
					},
					Volume: &StorageVolumeSource{
						PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
							ClaimName: restore.StagingPVCKey().Name,
						},
					},
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
							StorageClassName: ptr.To("my-sc"),
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("1Gi"),
								},
							},
						},
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
					Volume: &StorageVolumeSource{
						EmptyDir: &EmptyDirVolumeSource{},
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
							Volume: &StorageVolumeSource{
								NFS: &NFSVolumeSource{
									Server: "test",
									Path:   "test",
								},
							},
						},
					},
				},
				&RestoreSource{
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
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
					Volume: &StorageVolumeSource{
						EmptyDir: &EmptyDirVolumeSource{},
					},
				},
				true,
				false,
			),
			Entry(
				"Backup priority over Volume",
				&RestoreSource{
					Volume: &StorageVolumeSource{
						NFS: &NFSVolumeSource{
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
					Volume: &StorageVolumeSource{
						EmptyDir: &EmptyDirVolumeSource{},
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
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
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
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
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
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
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
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
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
				"AntiAffinity and nodeAffinity",
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						NodeAffinity: &NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &NodeSelector{
								NodeSelectorTerms: []NodeSelectorTerm{
									{
										MatchExpressions: []NodeSelectorRequirement{
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
				},
				[]string{"mariadb"},
				&AffinityConfig{
					AntiAffinityEnabled: ptr.To(true),
					Affinity: Affinity{
						PodAntiAffinity: &PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
								{
									LabelSelector: &LabelSelector{
										MatchExpressions: []LabelSelectorRequirement{
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
						NodeAffinity: &NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &NodeSelector{
								NodeSelectorTerms: []NodeSelectorTerm{
									{
										MatchExpressions: []NodeSelectorRequirement{
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
