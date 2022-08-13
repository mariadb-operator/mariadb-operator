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
)

var _ = Describe("BackupMariaDB webhook", func() {
	Context("When updating a BackupMariaDB", func() {
		It("Should validate", func() {
			By("Creating BackupMariaDB")
			key := types.NamespacedName{
				Name:      "backup-mariadb-webhook",
				Namespace: testNamespace,
			}
			initialBackup := BackupMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: BackupMariaDBSpec{
					Storage: Storage{
						ClassName: "standard",
						Size:      resource.MustParse("100Mi"),
					},
					MariaDBRef: corev1.LocalObjectReference{
						Name: "mariadb-webhook",
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
			Expect(k8sClient.Create(testCtx, &initialBackup)).To(Succeed())

			By("Updating BackoffLimit")
			backup := initialBackup.DeepCopy()
			backup.Spec.BackoffLimit = 20
			Expect(k8sClient.Update(testCtx, backup)).To(Succeed())

			By("Updating Storage")
			backup = initialBackup.DeepCopy()
			backup.Spec.Storage = Storage{
				ClassName: "fast-storage",
				Size:      resource.MustParse("200Mi"),
			}
			err := k8sClient.Update(testCtx, backup)
			Expect(err).To(HaveOccurred())

			By("Updating MariaDBRef")
			backup = initialBackup.DeepCopy()
			backup.Spec.MariaDBRef = corev1.LocalObjectReference{
				Name: "another-mariadb",
			}
			err = k8sClient.Update(testCtx, backup)
			Expect(err).To(HaveOccurred())

			By("Updating RestartPolicy")
			backup = initialBackup.DeepCopy()
			backup.Spec.RestartPolicy = corev1.RestartPolicyNever
			err = k8sClient.Update(testCtx, backup)
			Expect(err).To(HaveOccurred())

			By("Updating Resources")
			backup = initialBackup.DeepCopy()
			backup.Spec.Resources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu": resource.MustParse("200m"),
				},
			}
			err = k8sClient.Update(testCtx, backup)
			Expect(err).To(HaveOccurred())
		})
	})
})
