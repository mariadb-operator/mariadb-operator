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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Backup webhook", func() {
	Context("When creating a Backup", func() {
		It("Should validate", func() {
			storageClassName := "standard"
			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				backup  Backup
				wantErr bool
			}{
				{
					by: "Creating a Backup with invalid storage",
					backup: Backup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "backup-invalid-storage",
							Namespace: testNamespace,
						},
						Spec: BackupSpec{
							Storage: BackupStorage{},
							MariaDBRef: MariaDBRef{
								LocalObjectReference: corev1.LocalObjectReference{
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
					wantErr: true,
				},
				{
					by: "Creating a Backup with invalid schedule",
					backup: Backup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "backup-invalid-schedule",
							Namespace: testNamespace,
						},
						Spec: BackupSpec{
							Schedule: &BackupSchedule{
								Cron: "foo",
							},
							Storage: BackupStorage{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &storageClassName,
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
								LocalObjectReference: corev1.LocalObjectReference{
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
					wantErr: true,
				},
				{
					by: "Creating a valid Backup",
					backup: Backup{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "backup-valid",
							Namespace: testNamespace,
						},
						Spec: BackupSpec{
							Schedule: &BackupSchedule{
								Cron: "*/1 * * * *",
							},
							Storage: BackupStorage{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &storageClassName,
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
								LocalObjectReference: corev1.LocalObjectReference{
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
					wantErr: false,
				},
			}

			for _, t := range tt {
				By(t.by)
				err := k8sClient.Create(testCtx, &t.backup)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
	Context("When updating a Backup", func() {
		It("Should validate", func() {
			By("Creating Backup")
			key := types.NamespacedName{
				Name:      "backup-update",
				Namespace: testNamespace,
			}
			storageClassName := "standard"
			backup := Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: BackupSpec{
					Storage: BackupStorage{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &storageClassName,
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
						LocalObjectReference: corev1.LocalObjectReference{
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

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *Backup)
				wantErr bool
			}{
				{
					by: "Updating BackoffLimit",
					patchFn: func(bmdb *Backup) {
						bmdb.Spec.BackoffLimit = 20
					},
					wantErr: false,
				},
				{
					by: "Updating Schedule",
					patchFn: func(bmdb *Backup) {
						bmdb.Spec.Schedule = &BackupSchedule{
							Cron: "*/1 * * * *",
						}
					},
					wantErr: false,
				},
				{
					by: "Updating MaxBackupRetainDays",
					patchFn: func(bmdb *Backup) {
						bmdb.Spec.MaxRetentionDays = 40
					},
					wantErr: true,
				},
				{
					by: "Updating Storage",
					patchFn: func(bmdb *Backup) {
						newStorageClass := "fast-storage"
						bmdb.Spec.Storage.PersistentVolumeClaim.StorageClassName = &newStorageClass
					},
					wantErr: true,
				},
				{
					by: "Updating MariaDBRef",
					patchFn: func(bmdb *Backup) {
						bmdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating RestartPolicy",
					patchFn: func(bmdb *Backup) {
						bmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
					},
					wantErr: true,
				},
				{
					by: "Updating Resources",
					patchFn: func(bmdb *Backup) {
						bmdb.Spec.Resources = &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						}
					},
					wantErr: true,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &backup)).To(Succeed())

				patch := client.MergeFrom(backup.DeepCopy())
				t.patchFn(&backup)

				err := k8sClient.Patch(testCtx, &backup, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
