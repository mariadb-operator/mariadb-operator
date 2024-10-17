package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Backup webhook", func() {
	Context("When creating a Backup", func() {
		DescribeTable(
			"Should validate",
			func(backup *Backup, wantErr bool) {
				err := k8sClient.Create(testCtx, backup)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No storage",
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-storage",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: CompressGzip,
						Storage:     BackupStorage{},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
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
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-storage",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: CompressGzip,
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &StorageVolumeSource{
								PersistentVolumeClaim: &PersistentVolumeClaimVolumeSource{
									ClaimName: "TEST",
								},
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
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
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-storage",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: CompressGzip,
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
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
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-storage",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Compression: CompressAlgorithm("foo"),
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
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
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-schedule",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &Schedule{
							Cron: "foo",
						},
						Compression: CompressGzip,
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
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
				"Invalid history limits",
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-history-limits",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &Schedule{
							Cron: "*/1 * * * *",
						},
						Compression: CompressGzip,
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
						CronJobTemplate: CronJobTemplate{
							SuccessfulJobsHistoryLimit: ptr.To[int32](-5),
							FailedJobsHistoryLimit:     ptr.To[int32](-5),
						},
					},
				},
				true,
			),
			Entry(
				"Invalid staging storage",
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-staging-storage",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &Schedule{
							Cron: "*/1 * * * *",
						},
						Compression: CompressGzip,
						Storage: BackupStorage{
							Volume: &StorageVolumeSource{
								EmptyDir: &EmptyDirVolumeSource{},
							},
						},
						StagingStorage: &BackupStagingStorage{
							PersistentVolumeClaim: &PersistentVolumeClaimSpec{
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
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
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
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-valid",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &Schedule{
							Cron: "*/1 * * * *",
						},
						Compression: CompressGzip,
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						StagingStorage: &BackupStagingStorage{
							PersistentVolumeClaim: &PersistentVolumeClaimSpec{
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
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit:  10,
						RestartPolicy: corev1.RestartPolicyOnFailure,
						CronJobTemplate: CronJobTemplate{
							SuccessfulJobsHistoryLimit: ptr.To[int32](5),
							FailedJobsHistoryLimit:     ptr.To[int32](5),
						},
					},
				},
				false,
			),
		)
	})

	Context("When updating a Backup", Ordered, func() {
		key := types.NamespacedName{
			Name:      "backup-update",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			backup := Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: BackupSpec{
					JobContainerTemplate: JobContainerTemplate{
						Resources: &ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
					MaxRetention: metav1.Duration{Duration: 12 * time.Hour},
					Compression:  CompressNone,
					Storage: BackupStorage{
						S3: &S3{
							Bucket:   "test",
							Endpoint: "test",
						},
					},
					StagingStorage: &BackupStagingStorage{
						PersistentVolumeClaim: &PersistentVolumeClaimSpec{
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
					MariaDBRef: MariaDBRef{
						ObjectReference: ObjectReference{
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
			func(patchFn func(backup *Backup), wantErr bool) {
				var backup Backup
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
				func(bmdb *Backup) {
					bmdb.Spec.BackoffLimit = 20
				},
				false,
			),
			Entry(
				"Updating Schedule",
				func(bmdb *Backup) {
					bmdb.Spec.Schedule = &Schedule{
						Cron: "*/1 * * * *",
					}
				},
				false,
			),
			Entry(
				"Updating SuccessfulJobsHistoryLimit",
				func(bmdb *Backup) {
					bmdb.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](5)
				},
				false,
			),
			Entry(
				"Updating with wrong SuccessfulJobsHistoryLimit",
				func(bmdb *Backup) {
					bmdb.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](-5)
				},
				true,
			),
			Entry(
				"Updating FailedJobsHistoryLimit",
				func(bmdb *Backup) {
					bmdb.Spec.FailedJobsHistoryLimit = ptr.To[int32](5)
				},
				false,
			),
			Entry(
				"Updating with wrong FailedJobsHistoryLimit",
				func(bmdb *Backup) {
					bmdb.Spec.FailedJobsHistoryLimit = ptr.To[int32](-5)
				},
				true,
			),
			Entry(
				"Updating MaxRetention",
				func(bmdb *Backup) {
					bmdb.Spec.MaxRetention = metav1.Duration{Duration: 24 * time.Hour}
				},
				true,
			),
			Entry(
				"Updating Compression",
				func(bmdb *Backup) {
					bmdb.Spec.Compression = CompressBzip2
				},
				false,
			),
			Entry(
				"Updating Storage",
				func(bmdb *Backup) {
					bmdb.Spec.Storage.S3.Bucket = "another-bucket"
				},
				true,
			),
			Entry(
				"Updating StagingStorage",
				func(bmdb *Backup) {
					bmdb.Spec.StagingStorage = nil
				},
				true,
			),
			Entry(
				"Updating MariaDBRef",
				func(bmdb *Backup) {
					bmdb.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating RestartPolicy",
				func(bmdb *Backup) {
					bmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
				},
				true,
			),
			Entry(
				"Updating Resources",
				func(bmdb *Backup) {
					bmdb.Spec.Resources = &ResourceRequirements{
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
