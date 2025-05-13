package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Restore types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "restore-obj",
		Namespace: testNamespace,
	}
	mdbObjMeta := metav1.ObjectMeta{
		Name:      "mdb-restore-obj",
		Namespace: testNamespace,
	}
	Context("When creating a Restore object", func() {
		DescribeTable(
			"Should default",
			func(restore *Restore, mariadb *MariaDB, expectedRestore *Restore) {
				restore.SetDefaults(mariadb)
				Expect(restore).To(BeEquivalentTo(expectedRestore))
			},
			Entry(
				"Empty",
				&Restore{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Anti affinity",
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
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
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
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
						BackoffLimit: 5,
					},
				},
			),
			Entry(
				"Full",
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("restore-test"),
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
						BackoffLimit: 3,
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						JobPodTemplate: JobPodTemplate{
							ServiceAccountName: ptr.To("restore-test"),
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
						BackoffLimit: 3,
					},
				},
			),
		)
	})
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
				wantBackupDefaultErr bool,
			) {
				rs.SetDefaults(&restore)
				if backup != nil {
					err := rs.SetDefaultsWithBackup(backup)
					if wantBackupDefaultErr {
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
})
