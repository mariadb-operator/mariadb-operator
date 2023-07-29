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

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Restore controller", func() {
	Context("When creating a restore", func() {
		It("Should reconcile", func() {
			By("Creating Backup")
			backupKey := types.NamespacedName{
				Name:      "restore-mariadb-test",
				Namespace: testNamespace,
			}
			backup := mariadbv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Storage: mariadbv1alpha1.BackupStorage{
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
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			By("Expecting Backup to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating a MariaDB")
			mariaDBKey := types.NamespacedName{
				Name:      "mariadb-restore",
				Namespace: testNamespace,
			}
			mariaDB := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mariaDBKey.Name,
					Namespace: mariaDBKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
			Expect(k8sClient.Create(testCtx, &mariaDB)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, mariaDBKey, &mariaDB); err != nil {
					return false
				}
				return mariaDB.IsReady()
			}, 60*time.Second, testInterval).Should(BeTrue())

			Expect(k8sClient.Get(testCtx, mariaDBKey, &mariaDB)).To(Succeed())

			By("Creating restore")
			restoreKey := types.NamespacedName{
				Name:      "restore-test",
				Namespace: testNamespace,
			}
			restore := mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
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

			By("Expecting restore to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, restoreKey, &restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, &backup)).To(Succeed())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &mariaDB)).To(Succeed())

			By("Deleting restore")
			Expect(k8sClient.Delete(testCtx, &restore)).To(Succeed())
		})
	})
})
