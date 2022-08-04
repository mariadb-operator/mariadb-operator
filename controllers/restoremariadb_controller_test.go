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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RestoreMariaDB controller", func() {
	Context("When creating a RestoreMariaDB", func() {
		It("Should reconcile", func() {
			By("Creating BackupMariaDB")
			backupKey := types.NamespacedName{
				Name:      "restore-mariadb-test",
				Namespace: defaultNamespace,
			}
			backup := databasev1alpha1.BackupMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: databasev1alpha1.BackupMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: mariaDbName,
					},
					Storage: databasev1alpha1.Storage{
						ClassName: defaultStorageClass,
						Size:      storageSize,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &backup)).To(Succeed())

			By("Expecting BackupMariaDB to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating a MariaDB")
			restoreMariaDbKey := types.NamespacedName{
				Name:      "mariadb-restore",
				Namespace: defaultNamespace,
			}
			restoreMariaDb := databasev1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreMariaDbKey.Name,
					Namespace: restoreMariaDbKey.Namespace,
				},
				Spec: databasev1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mariaDbRootPwdKey.Name,
						},
						Key: mariaDbRootPwdSecretKey,
					},
					Image: databasev1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					Storage: databasev1alpha1.Storage{
						ClassName: defaultStorageClass,
						Size:      storageSize,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &restoreMariaDb)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, restoreMariaDbKey, &restoreMariaDb); err != nil {
					return false
				}
				return restoreMariaDb.IsReady()
			}, 60*time.Second, testInterval).Should(BeTrue())

			Expect(k8sClient.Get(ctx, restoreMariaDbKey, &restoreMariaDb)).To(Succeed())

			By("Creating RestoreMariaDB")
			restoreKey := types.NamespacedName{
				Name:      "restore-test",
				Namespace: defaultNamespace,
			}
			restore := databasev1alpha1.RestoreMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: mariaDbName,
					},
					BackupRef: corev1.LocalObjectReference{
						Name: backup.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &restore)).To(Succeed())

			By("Expecting to create a Job eventually")
			Eventually(func() bool {
				var job batchv1.Job
				if err := k8sClient.Get(ctx, restoreKey, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting RestoreMariaDB to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, restoreKey, &restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting BackupMariaDB")
			Expect(k8sClient.Delete(ctx, &backup)).To(Succeed())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(ctx, &restoreMariaDb)).To(Succeed())

			By("Deleting RestoreMariaDB")
			Expect(k8sClient.Delete(ctx, &restore)).To(Succeed())
		})
	})
})
