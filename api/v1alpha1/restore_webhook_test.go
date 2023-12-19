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
				"Invalid source 1",
				&Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore-invalid-source-1",
						Namespace: testNamespace,
					},
					Spec: RestoreSpec{
						RestoreSource: RestoreSource{},
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
				"Invalid source 2",
				&Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore-invalid-source-2",
						Namespace: testNamespace,
					},
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
				"Valid source 1",
				&Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore-webhook-1",
						Namespace: testNamespace,
					},
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
				"Valid source 2",
				&Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore-webhook-2",
						Namespace: testNamespace,
					},
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
				"Valid source 3",
				&Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore-webhook-3",
						Namespace: testNamespace,
					},
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
		)
	})

	Context("When updating a Restore", Ordered, func() {
		key := types.NamespacedName{
			Name:      "restore-mariadb-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			restore := Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
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
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("100m"),
						},
					},
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
				true,
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
				"Init Volume source",
				func(rmdb *Restore) {
					rmdb.Spec.RestoreSource.Volume = &corev1.VolumeSource{
						NFS: &corev1.NFSVolumeSource{
							Server: "nas.local",
							Path:   "/volume/foo",
						},
					}
				},
				false,
			),
			Entry(
				"Init TargetRecoveryTime source",
				func(rmdb *Restore) {
					rmdb.Spec.RestoreSource.TargetRecoveryTime = &metav1.Time{Time: time.Now().Add(1 * time.Hour)}
				},
				false,
			),
		)
	})
})
