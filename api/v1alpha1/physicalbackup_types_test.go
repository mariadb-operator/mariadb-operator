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

var _ = Describe("PhysicalBackup types", func() {
	objMeta := metav1.ObjectMeta{
		Name:      "physicalbackup-obj",
		Namespace: testNamespace,
	}
	backup := PhysicalBackup{
		ObjectMeta: objMeta,
	}
	mdbObjMeta := metav1.ObjectMeta{
		Name:      "mdb-physicalbackup-obj",
		Namespace: testNamespace,
	}
	Context("When creating a PhysicalBackup object", func() {
		DescribeTable(
			"Should default",
			func(backup *PhysicalBackup, mariadb *MariaDB, expectedBackup *PhysicalBackup) {
				backup.SetDefaults(mariadb)
				Expect(backup).To(BeEquivalentTo(expectedBackup))
			},
			Entry(
				"Empty",
				&PhysicalBackup{
					ObjectMeta: objMeta,
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						PhysicalBackupPodTemplate: PhysicalBackupPodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Target:                     ptr.To(PhysicalBackupTargetReplica),
						Compression:                CompressNone,
						MaxRetention:               DefaultPhysicalBackupMaxRetention,
						Timeout:                    &DefaultPhysicalBackupTimeout,
						BackoffLimit:               5,
						SuccessfulJobsHistoryLimit: ptr.To(int32(5)),
					},
				},
			),
			Entry(
				"Already defaulted",
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						PhysicalBackupPodTemplate: PhysicalBackupPodTemplate{
							ServiceAccountName: ptr.To("physicalbackup-test"),
						},
						Target:                     ptr.To(PhysicalBackupTargetPreferReplica),
						Compression:                CompressBzip2,
						MaxRetention:               metav1.Duration{Duration: 10 * 24 * time.Hour},
						Timeout:                    &metav1.Duration{Duration: 5 * time.Minute},
						BackoffLimit:               3,
						SuccessfulJobsHistoryLimit: ptr.To(int32(3)),
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						PhysicalBackupPodTemplate: PhysicalBackupPodTemplate{
							ServiceAccountName: ptr.To("physicalbackup-test"),
						},
						Target:                     ptr.To(PhysicalBackupTargetPreferReplica),
						Compression:                CompressBzip2,
						MaxRetention:               metav1.Duration{Duration: 10 * 24 * time.Hour},
						Timeout:                    &metav1.Duration{Duration: 5 * time.Minute},
						BackoffLimit:               3,
						SuccessfulJobsHistoryLimit: ptr.To(int32(3)),
					},
				},
			),
		)
		DescribeTable(
			"Should default VolumeSnapshot",
			func(backup *PhysicalBackup, mariadb *MariaDB, expectedBackup *PhysicalBackup) {
				backup.SetDefaults(mariadb)
				Expect(backup).To(BeEquivalentTo(expectedBackup))
			},
			Entry(
				"Empty",
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
							VolumeSnapshot: &PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Target: ptr.To(PhysicalBackupTargetReplica),
						Storage: PhysicalBackupStorage{
							VolumeSnapshot: &PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
						MaxRetention: DefaultPhysicalBackupMaxRetention,
						Timeout:      &DefaultPhysicalBackupTimeout,
					},
				},
			),
			Entry(
				"Already defaulted",
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Target: ptr.To(PhysicalBackupTargetPreferReplica),
						Storage: PhysicalBackupStorage{
							VolumeSnapshot: &PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
						MaxRetention: metav1.Duration{Duration: 10 * 24 * time.Hour},
						Timeout:      &metav1.Duration{Duration: 5 * time.Minute},
					},
				},
				&MariaDB{
					ObjectMeta: mdbObjMeta,
				},
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Target: ptr.To(PhysicalBackupTargetPreferReplica),
						Storage: PhysicalBackupStorage{
							VolumeSnapshot: &PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
						MaxRetention: metav1.Duration{Duration: 10 * 24 * time.Hour},
						Timeout:      &metav1.Duration{Duration: 5 * time.Minute},
					},
				},
			),
		)
		DescribeTable(
			"Should return a volume",
			func(backup *PhysicalBackup, expectedVolume StorageVolumeSource, wantErr bool) {
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
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{},
					},
				},
				nil,
				true,
			),
			Entry(
				"VolumeSnapshot not compatible with volumes",
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
							VolumeSnapshot: &PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
					},
				},
				nil,
				true,
			),
			Entry(
				"S3",
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
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
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
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
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
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
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
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
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
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
				&PhysicalBackup{
					ObjectMeta: objMeta,
					Spec: PhysicalBackupSpec{
						Storage: PhysicalBackupStorage{
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
