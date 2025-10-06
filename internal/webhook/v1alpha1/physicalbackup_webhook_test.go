package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PhysicalBackup webhook", func() {
	Context("When creating a PhysicalBackup", func() {
		DescribeTable(
			"Should validate",
			func(backup *v1alpha1.PhysicalBackup, wantErr bool) {
				err := k8sClient.Create(testCtx, backup)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No storage",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-storage",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: v1alpha1.CompressGzip,
						Storage:     v1alpha1.PhysicalBackupStorage{},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				true,
			),
			Entry(
				"Multiple storages",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-multiple-storages",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &v1alpha1.StorageVolumeSource{
								PersistentVolumeClaim: &v1alpha1.PersistentVolumeClaimVolumeSource{
									ClaimName: "TEST",
								},
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				true,
			),
			Entry(
				"Single storage",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-single-storage",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				false,
			),
			Entry(
				"Invalid compression",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-compression",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: v1alpha1.CompressAlgorithm("foo"),
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				true,
			),
			Entry(
				"Invalid cron",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-schedule",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &v1alpha1.PhysicalBackupSchedule{
							Cron: "foo",
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				true,
			),
			Entry(
				"Invalid schedule",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-schedule",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &v1alpha1.PhysicalBackupSchedule{
							Cron:    "",
							Suspend: false,
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				true,
			),
			Entry(
				"Suspended schedule",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-schedule",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &v1alpha1.PhysicalBackupSchedule{
							Suspend: true,
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				false,
			),
			Entry(
				"Invalid history limits",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-history-limits",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &v1alpha1.PhysicalBackupSchedule{
							Cron: "*/1 * * * *",
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:               10,
						RestartPolicy:              corev1.RestartPolicyOnFailure,
						SuccessfulJobsHistoryLimit: ptr.To[int32](-5),
					},
				},
				true,
			),
			Entry(
				"Invalid volume snapshot 1",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-volumesnapshot-1",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							VolumeSnapshot: &v1alpha1.PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid volume snapshot 2",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-volumesnapshot-2",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							Volume: &v1alpha1.StorageVolumeSource{
								PersistentVolumeClaim: &v1alpha1.PersistentVolumeClaimVolumeSource{
									ClaimName: "test",
								},
							},
							VolumeSnapshot: &v1alpha1.PhysicalBackupVolumeSnapshot{
								VolumeSnapshotClassName: "test",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
					},
				},
				true,
			),
			Entry(
				"Invalid staging storage",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-invalid-staging-storage",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &v1alpha1.PhysicalBackupSchedule{
							Cron: "*/1 * * * *",
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							Volume: &v1alpha1.StorageVolumeSource{
								EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
							},
						},
						StagingStorage: &v1alpha1.BackupStagingStorage{
							PersistentVolumeClaim: &v1alpha1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("300Mi"),
									},
								},
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
					},
				},
				true,
			),
			Entry(
				"Valid",
				&v1alpha1.PhysicalBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "physicalbackup-valid",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.PhysicalBackupSpec{
						JobContainerTemplate: v1alpha1.JobContainerTemplate{
							Resources: &v1alpha1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &v1alpha1.PhysicalBackupSchedule{
							Cron: "*/1 * * * *",
						},
						Compression: v1alpha1.CompressGzip,
						Storage: v1alpha1.PhysicalBackupStorage{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						StagingStorage: &v1alpha1.BackupStagingStorage{
							PersistentVolumeClaim: &v1alpha1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("300Mi"),
									},
								},
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:               10,
						RestartPolicy:              corev1.RestartPolicyOnFailure,
						SuccessfulJobsHistoryLimit: ptr.To[int32](5),
					},
				},
				false,
			),
		)
	})

	Context("When updating a PhysicalBackup", Ordered, func() {
		key := types.NamespacedName{
			Name:      "physicalbackup-update",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			backup := v1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.PhysicalBackupSpec{
					JobContainerTemplate: v1alpha1.JobContainerTemplate{
						Resources: &v1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
					MaxRetention: metav1.Duration{Duration: 12 * time.Hour},
					Compression:  v1alpha1.CompressNone,
					Storage: v1alpha1.PhysicalBackupStorage{
						S3: &v1alpha1.S3{
							Bucket:   "test",
							Endpoint: "test",
						},
					},
					Schedule: &v1alpha1.PhysicalBackupSchedule{
						Cron: "* */1 * * *",
					},
					StagingStorage: &v1alpha1.BackupStagingStorage{
						PersistentVolumeClaim: &v1alpha1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("300Mi"),
								},
							},
						},
					},
					MariaDBRef: v1alpha1.MariaDBRef{
						ObjectReference: v1alpha1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					BackoffLimit:  10,
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())
		})

		DescribeTable(
			"Should validate",
			func(patchFn func(backup *v1alpha1.PhysicalBackup), wantErr bool) {
				var backup v1alpha1.PhysicalBackup
				Expect(k8sClient.Get(testCtx, key, &backup)).To(Succeed())

				patch := client.MergeFrom(backup.DeepCopy())
				patchFn(&backup)

				err := k8sClient.Patch(testCtx, &backup, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating BackoffLimit",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.BackoffLimit = 20
				},
				false,
			),
			Entry(
				"Updating Schedule",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.Schedule = &v1alpha1.PhysicalBackupSchedule{
						Cron: "*/1 * * * *",
					}
				},
				true,
			),
			Entry(
				"Updating SuccessfulJobsHistoryLimit",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](5)
				},
				false,
			),
			Entry(
				"Updating with wrong SuccessfulJobsHistoryLimit",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](-5)
				},
				true,
			),
			Entry(
				"Updating MaxRetention",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.MaxRetention = metav1.Duration{Duration: 24 * time.Hour}
				},
				false,
			),
			Entry(
				"Updating Compression",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.Compression = v1alpha1.CompressBzip2
				},
				false,
			),
			Entry(
				"Updating Storage",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.Storage.S3.Bucket = "another-bucket"
				},
				true,
			),
			Entry(
				"Updating StagingStorage",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.StagingStorage = nil
				},
				true,
			),
			Entry(
				"Updating MariaDBRef",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating RestartPolicy",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
				},
				true,
			),
			Entry(
				"Updating Resources",
				func(bmdb *v1alpha1.PhysicalBackup) {
					bmdb.Spec.Resources = &v1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
		)
	})
})
