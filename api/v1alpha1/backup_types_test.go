package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Backup types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "backup-obj",
		Namespace: testNamespace,
	}
	backup := Backup{
		ObjectMeta: objMeta,
	}
	mdbObjMeta := metav1.ObjectMeta{
		Name:      "mdb-backup-obj",
		Namespace: testNamespace,
	}
	Context("When creating a Backup object", func() {
		DescribeTable(
			"Should default",
			func(backup *Backup, mariadb *MariaDB, expectedBackup *Backup) {
				backup.SetDefaults(mariadb)
				Expect(backup).To(BeEquivalentTo(expectedBackup))
			},
			Entry(
				"Empty",
				&Backup{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Compression:      CompressNone,
						MaxRetention:     metav1.Duration{Duration: 30 * 24 * time.Hour},
						IgnoreGlobalPriv: ptr.To(false),
						BackoffLimit:     5,
					},
				},
			),
			Entry(
				"Galera",
				&Backup{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
						},
					},
				},
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Compression:      CompressNone,
						MaxRetention:     metav1.Duration{Duration: 30 * 24 * time.Hour},
						IgnoreGlobalPriv: ptr.To(true),
						BackoffLimit:     5,
					},
				},
			),
			Entry(
				"Anti affinity",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						JobPodTemplate: JobPodTemplate{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
							Affinity: &AffinityConfig{
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
															Values: []string{
																mdbObjMeta.Name,
															},
														},
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
						Compression:      CompressNone,
						MaxRetention:     metav1.Duration{Duration: 30 * 24 * time.Hour},
						IgnoreGlobalPriv: ptr.To(false),
						BackoffLimit:     5,
					},
				},
			),
			Entry(
				"Full",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("backup-test"),
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
						Compression:      CompressBzip2,
						MaxRetention:     metav1.Duration{Duration: 10 * 24 * time.Hour},
						IgnoreGlobalPriv: ptr.To(false),
						BackoffLimit:     3,
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
					Spec: MariaDBSpec{
						Galera: &Galera{
							Enabled: true,
						},
					},
				},
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("backup-test"),
							Affinity: &AffinityConfig{
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
															Values: []string{
																mdbObjMeta.Name,
															},
														},
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
						Compression:      CompressBzip2,
						MaxRetention:     metav1.Duration{Duration: 10 * 24 * time.Hour},
						IgnoreGlobalPriv: ptr.To(false),
						BackoffLimit:     3,
					},
				},
			),
		)
		DescribeTable(
			"Should return a volume",
			func(backup *Backup, expectedVolume StorageVolumeSource, wantErr bool) {
				volume, err := backup.Volume()
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				Expect(volume).To(BeEquivalentTo(expectedVolume))
			},
			Entry(
				"No storage",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						Storage: BackupStorage{},
					},
				},
				nil,
				true,
			),
			Entry(
				"S3",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{},
						},
					},
				},
				StorageVolumeSource{
					EmptyDir: &EmptyDirVolumeSource{},
				},
				false,
			),
			Entry(
				"S3 with staging",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{},
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
				},
				StorageVolumeSource{
					PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
						ClaimName: backup.StagingPVCKey().Name,
					},
				},
				false,
			),
			Entry(
				"PVC",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						Storage: BackupStorage{
							PersistentVolumeClaim: &PersistentVolumeClaimSpec{},
						},
					},
				},
				StorageVolumeSource{
					PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
						ClaimName: objMeta.Name,
					},
				},
				false,
			),
			Entry(
				"Volume",
				&Backup{
					ObjectMeta: objMeta,
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
				StorageVolumeSource{
					NFS: &NFSVolumeSource{
						Server: "test",
						Path:   "test",
					},
				},
				false,
			),
			Entry(
				"S3 priority over Volume",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{},
							Volume: &StorageVolumeSource{
								NFS: &NFSVolumeSource{
									Server: "test",
									Path:   "test",
								},
							},
						},
					},
				},
				StorageVolumeSource{
					EmptyDir: &EmptyDirVolumeSource{},
				},
				false,
			),
			Entry(
				"S3 with staging priority over Volume",
				&Backup{
					ObjectMeta: objMeta,
					Spec: BackupSpec{
						Storage: BackupStorage{
							S3: &S3{},
							Volume: &StorageVolumeSource{
								NFS: &NFSVolumeSource{
									Server: "test",
									Path:   "test",
								},
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
				},
				StorageVolumeSource{
					PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
						ClaimName: backup.StagingPVCKey().Name,
					},
				},
				false,
			),
		)
	})
})
