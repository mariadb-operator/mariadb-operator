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

var _ = Describe("DatabaseMariaDB webhook", func() {
	Context("When updating a DatabaseMariaDB", func() {
		It("Should validate", func() {
			By("Creating DatabaseMariaDB")
			key := types.NamespacedName{
				Name:      "database-mariadb-webhook",
				Namespace: testNamespace,
			}
			initialDatabase := DatabaseMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: DatabaseMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: "mariadb-webhook",
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(testCtx, &initialDatabase)).To(Succeed())

			By("Updating MariaDBRef")
			database := initialDatabase.DeepCopy()
			database.Spec.MariaDBRef = corev1.LocalObjectReference{
				Name: "another-mariadb",
			}
			err := k8sClient.Update(testCtx, database)
			Expect(err).To(HaveOccurred())

			By("Updating CharacterSet")
			database = initialDatabase.DeepCopy()
			database.Spec.CharacterSet = "utf16"
			err = k8sClient.Update(testCtx, database)
			Expect(err).To(HaveOccurred())

			By("Updating Collate")
			database = initialDatabase.DeepCopy()
			database.Spec.Collate = "latin2_general_ci"
			err = k8sClient.Update(testCtx, database)
			Expect(err).To(HaveOccurred())
		})
	})
})
