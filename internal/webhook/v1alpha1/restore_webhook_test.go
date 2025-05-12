package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Restore webhook", func() {
	Context("When creating a v1alpha1.Restore", func() {
		objMeta := metav1.ObjectMeta{
			Name:      "restore-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(restore *v1alpha1.Restore, wantErr bool) {
				_ = k8sClient.Delete(testCtx, restore)
				err := k8sClient.Create(testCtx, restore)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No source",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							TargetRecoveryTime: &metav1.Time{Time: time.Now()},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				true,
			),
			Entry(
				"BackupRef source",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							BackupRef: &v1alpha1.LocalObjectReference{
								Name: "backup-webhook",
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"S3 source",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
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
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"Volume source",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							Volume: &v1alpha1.StorageVolumeSource{
								PersistentVolumeClaim: &v1alpha1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-webhook",
								},
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"S3 and Volume source",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &v1alpha1.StorageVolumeSource{
								EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"BackupRef, S3 and Volume source",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							BackupRef: &v1alpha1.LocalObjectReference{
								Name: "backup-webhook",
							},
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &v1alpha1.StorageVolumeSource{
								EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
							},
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"S3 and staging storage",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							S3: &v1alpha1.S3{
								Bucket:   "test",
								Endpoint: "test",
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
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"Volume and stagingStorage",
				&v1alpha1.Restore{
					ObjectMeta: objMeta,
					Spec: v1alpha1.RestoreSpec{
						RestoreSource: v1alpha1.RestoreSource{
							Volume: &v1alpha1.StorageVolumeSource{
								EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
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
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				true,
			),
		)
	})

	Context("When updating a v1alpha1.Restore", Ordered, func() {
		key := types.NamespacedName{
			Name:      "restore-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			restore := v1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.RestoreSpec{
					JobContainerTemplate: v1alpha1.JobContainerTemplate{
						Resources: &v1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
					RestoreSource: v1alpha1.RestoreSource{
						S3: &v1alpha1.S3{
							Bucket:   "test",
							Endpoint: "test",
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
						TargetRecoveryTime: &metav1.Time{Time: time.Now()},
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
			Expect(k8sClient.Create(testCtx, &restore)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(restore *v1alpha1.Restore), wantErr bool) {
				var restore v1alpha1.Restore
				Expect(k8sClient.Get(testCtx, key, &restore)).To(Succeed())

				patch := client.MergeFrom(restore.DeepCopy())
				patchFn(&restore)

				err := k8sClient.Patch(testCtx, &restore, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating BackoffLimit",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.BackoffLimit = 20
				},
				false,
			),
			Entry(
				"Updating RestartPolicy",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
				},
				true,
			),
			Entry(
				"Updating Resources",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.Resources = &v1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
			Entry(
				"Updating MariaDBRef",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating BackupRef source",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.RestoreSource.BackupRef = &v1alpha1.LocalObjectReference{
						Name: "backup",
					}
				},
				false,
			),
			Entry(
				"Update S3 source",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.RestoreSource.S3 = &v1alpha1.S3{
						Bucket:   "another-bucket",
						Endpoint: "another-endpoint",
					}
				},
				true,
			),
			Entry(
				"Init Volume source",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.RestoreSource.Volume = &v1alpha1.StorageVolumeSource{
						EmptyDir: &v1alpha1.EmptyDirVolumeSource{},
					}
				},
				false,
			),
			Entry(
				"Init TargetRecoveryTime source",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.RestoreSource.TargetRecoveryTime = &metav1.Time{Time: time.Now().Add(1 * time.Hour)}
				},
				true,
			),
			Entry(
				"Update StagingStorage",
				func(rmdb *v1alpha1.Restore) {
					rmdb.Spec.StagingStorage = nil
				},
				true,
			),
		)
	})
})
