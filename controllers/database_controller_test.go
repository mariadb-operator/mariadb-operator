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
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Database controller", func() {
	Context("When creating a Database", func() {
		It("Should reconcile", func() {
			By("Creating a Database")
			databaseKey := types.NamespacedName{
				Name:      "data-test",
				Namespace: testNamespace,
			}
			database := mariadbv1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      databaseKey.Name,
					Namespace: databaseKey.Namespace,
				},
				Spec: mariadbv1alpha1.DatabaseSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbKey.Name,
						},
						WaitForIt: true,
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(testCtx, &database)).To(Succeed())

			By("Expecting Database to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
					return false
				}
				return database.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting Database to eventually have finalizer")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(&database, databaseFinalizerName)
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Database")
			Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		})
	})
})
