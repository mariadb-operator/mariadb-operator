/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
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
				"Invalid storage",
				&Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup-invalid-storage",
						Namespace: testNamespace,
					},
					Spec: BackupSpec{
						Storage: BackupStorage{},
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
						Schedule: &Schedule{
							Cron: "foo",
						},
						Storage: BackupStorage{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("100Mi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
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
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
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
						Schedule: &Schedule{
							Cron: "*/1 * * * *",
						},
						Storage: BackupStorage{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": resource.MustParse("100Mi"),
									},
								},
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
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
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
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
					Storage: BackupStorage{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("100Mi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
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
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("100m"),
						},
					},
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
				"Updating MaxBackupRetainDays",
				func(bmdb *Backup) {
					bmdb.Spec.MaxRetentionDays = 40
				},
				true,
			),
			Entry(
				"Updating Storage",
				func(bmdb *Backup) {
					newStorageClass := "fast-storage"
					bmdb.Spec.Storage.PersistentVolumeClaim.StorageClassName = &newStorageClass
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
				true,
			),
		)
	})
})
