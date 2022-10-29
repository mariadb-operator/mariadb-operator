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

package controllers

import (
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RestoreMariaDB controller", func() {
	Context("When creating a RestoreMariaDB", func() {
		It("Should reconcile", func() {
			By("Creating BackupMariaDB")
			backupKey := types.NamespacedName{
				Name:      "restore-mariadb-test",
				Namespace: testNamespace,
			}
			backup := databasev1alpha1.BackupMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: databasev1alpha1.BackupMariaDBSpec{
					MariaDBRef: databasev1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Storage: databasev1alpha1.Storage{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &testStorageClassName,
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
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			By("Expecting BackupMariaDB to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating a MariaDB")
			restoreMariaDbKey := types.NamespacedName{
				Name:      "mariadb-restore",
				Namespace: testNamespace,
			}
			restoreMariaDb := databasev1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreMariaDbKey.Name,
					Namespace: restoreMariaDbKey.Namespace,
				},
				Spec: databasev1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testRootPwdKey.Name,
						},
						Key: testRootPwdSecretKey,
					},
					Image: databasev1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &testStorageClassName,
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
			}
			Expect(k8sClient.Create(testCtx, &restoreMariaDb)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, restoreMariaDbKey, &restoreMariaDb); err != nil {
					return false
				}
				return restoreMariaDb.IsReady()
			}, 60*time.Second, testInterval).Should(BeTrue())

			Expect(k8sClient.Get(testCtx, restoreMariaDbKey, &restoreMariaDb)).To(Succeed())

			By("Creating RestoreMariaDB")
			restoreKey := types.NamespacedName{
				Name:      "restore-test",
				Namespace: testNamespace,
			}
			restore := databasev1alpha1.RestoreMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					MariaDBRef: databasev1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					BackupRef: databasev1alpha1.BackupMariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: backup.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &restore)).To(Succeed())

			By("Expecting to create a Job eventually")
			Eventually(func() bool {
				var job batchv1.Job
				if err := k8sClient.Get(testCtx, restoreKey, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting RestoreMariaDB to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, restoreKey, &restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting BackupMariaDB")
			Expect(k8sClient.Delete(testCtx, &backup)).To(Succeed())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &restoreMariaDb)).To(Succeed())

			By("Deleting RestoreMariaDB")
			Expect(k8sClient.Delete(testCtx, &restore)).To(Succeed())
		})
	})
})
