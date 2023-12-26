package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
})
