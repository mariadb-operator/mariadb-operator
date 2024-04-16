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

var _ = Describe("Restore webhook", func() {
	Context("When creating a Restore", func() {
		objMeta := metav1.ObjectMeta{
			Name:      "restore-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(restore *Restore, wantErr bool) {
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
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{
							TargetRecoveryTime: &metav1.Time{Time: time.Now()},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{
							BackupRef: &corev1.LocalObjectReference{
								Name: "backup-webhook",
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{
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
						BackoffLimit: 10,
					},
				},
				false,
			),
			Entry(
				"Volume source",
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{
							Volume: &corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-webhook",
								},
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
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
				&Restore{
					ObjectMeta: objMeta,
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{
							BackupRef: &corev1.LocalObjectReference{
								Name: "backup-webhook",
							},
							S3: &S3{
								Bucket:   "test",
								Endpoint: "test",
							},
							Volume: &corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						BackoffLimit: 10,
					},
				},
				false,
			),
		)
	})

	Context("When updating a Restore", Ordered, func() {
		key := types.NamespacedName{
			Name:      "restore-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			restore := Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: RestoreSpec{
					JobContainerTemplate: JobContainerTemplate{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
					},
					RestoreSource: RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "backup-webhook",
						},
						TargetRecoveryTime: &metav1.Time{Time: time.Now()},
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
			Expect(k8sClient.Create(testCtx, &restore)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(restore *Restore), wantErr bool) {
				var restore Restore
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
				func(rmdb *Restore) {
					rmdb.Spec.BackoffLimit = 20
				},
				false,
			),
			Entry(
				"Updating RestartPolicy",
				func(rmdb *Restore) {
					rmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
				},
				true,
			),
			Entry(
				"Updating Resources",
				func(rmdb *Restore) {
					rmdb.Spec.Resources = &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
			Entry(
				"Updating MariaDBRef",
				func(rmdb *Restore) {
					rmdb.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating BackupRef source",
				func(rmdb *Restore) {
					rmdb.Spec.RestoreSource.BackupRef.Name = "another-backup"
				},
				true,
			),
			Entry(
				"Init S3 source",
				func(rmdb *Restore) {
					rmdb.Spec.RestoreSource.S3 = &S3{
						Bucket:   "test",
						Endpoint: "test",
					}
				},
				false,
			),
			Entry(
				"Init Volume source",
				func(rmdb *Restore) {
					rmdb.Spec.RestoreSource.Volume = &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					}
				},
				false,
			),
			Entry(
				"Init TargetRecoveryTime source",
				func(rmdb *Restore) {
					rmdb.Spec.RestoreSource.TargetRecoveryTime = &metav1.Time{Time: time.Now().Add(1 * time.Hour)}
				},
				true,
			),
		)
	})
})
