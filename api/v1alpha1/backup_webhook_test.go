package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Storage: BackupStorage{},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "TEST",
								},
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
				"Invalid schedule",
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-schedule",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						JobContainerTemplate: JobContainerTemplate{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &Schedule{
							Cron: "foo",
						},
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("100m"),
								},
							},
						},
						Schedule: &Schedule{
							Cron: "*/1 * * * *",
						},
						Storage: BackupStorage{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
					MaxRetention: metav1.Duration{Duration: 12 * time.Hour},
					Storage: BackupStorage{
						S3: &S3{
							Bucket:   "test",
							Endpoint: "test",
						},
					},
					MariaDBRef: MariaDBRef{
						ObjectReference: corev1.ObjectReference{
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
				"Updating MaxRetention",
				func(bmdb *Backup) {
					bmdb.Spec.MaxRetention = metav1.Duration{Duration: 24 * time.Hour}
				},
				true,
			),
			Entry(
				"Updating Storage",
				func(bmdb *Backup) {
					bmdb.Spec.Storage.S3.Bucket = "another-bucket"
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
					bmdb.Spec.Resources = &corev1.ResourceRequirements{
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
