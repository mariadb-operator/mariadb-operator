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
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("DatabaseMariaDB controller", func() {
	Context("When creating a DatabaseMariaDB", func() {
		It("Should reconcile", func() {
			By("Creating a DatabaseMariaDB")
			databaseKey := types.NamespacedName{
				Name:      "data-test",
				Namespace: testNamespace,
			}
			database := databasev1alpha1.DatabaseMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      databaseKey.Name,
					Namespace: databaseKey.Namespace,
				},
				Spec: databasev1alpha1.DatabaseMariaDBSpec{
					MariaDBRef: databasev1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbKey.Name,
						},
						WaitForIt: true,
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(testCtx, &database)).To(Succeed())

			By("Expecting DatabaseMariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
					return false
				}
				return database.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			Expect(database.ObjectMeta.Finalizers).To(ContainElement(databaseFinalizerName))

			By("Deleting DatabaseMariaDB")
			Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		})
	})
})
