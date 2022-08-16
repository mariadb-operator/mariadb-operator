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
			initialUser := UserMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: UserMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: "mariadb-webhook",
					},
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "backup-mariadb-webhook-root",
						},
						Key: "passowrd",
					},
					MaxUserConnections: 10,
				},
			}
			Expect(k8sClient.Create(testCtx, &initialUser)).To(Succeed())

			By("Updating MariaDBRef")
			user := initialUser.DeepCopy()
			user.Spec.MariaDBRef = corev1.LocalObjectReference{
				Name: "another-mariadb",
			}
			err := k8sClient.Update(testCtx, user)
			Expect(err).To(HaveOccurred())

			By("Updating PasswordSecretKeyRef")
			user = initialUser.DeepCopy()
			user.Spec.MariaDBRef = corev1.LocalObjectReference{
				Name: "another-mariadb",
			}
			err = k8sClient.Update(testCtx, user)
			Expect(err).To(HaveOccurred())

			By("Updating MaxUserConnections")
			user = initialUser.DeepCopy()
			user.Spec.MaxUserConnections = 20
			err = k8sClient.Update(testCtx, user)
			Expect(err).To(HaveOccurred())
		})
	})
})
