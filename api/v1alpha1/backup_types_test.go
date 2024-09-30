package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Backup types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "backup-obj",
		Namespace: testNamespace,
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
												LabelSelector: &metav1.LabelSelector{
													MatchExpressions: []metav1.LabelSelectorRequirement{
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
												LabelSelector: &metav1.LabelSelector{
													MatchExpressions: []metav1.LabelSelectorRequirement{
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
			func(backup *Backup, expectedVolume corev1.VolumeSource, wantErr bool) {
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
				corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
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
				corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
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
							Volume: &VolumeSource{
								NFS: &corev1.NFSVolumeSource{
									Server: "test",
									Path:   "test",
								},
							},
						},
					},
				},
				corev1.VolumeSource{
					NFS: &corev1.NFSVolumeSource{
						Server: "test",
						Path:   "test",
					},
				},
				false,
			),
		)
	})
})
