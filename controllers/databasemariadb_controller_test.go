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
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/portforwarder"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("DatabaseMariaDB controller", func() {
	Context("When creating a DatabaseMariaDB", func() {
		It("Should reconcile", func() {
			By("Creating a port forward to MariaDB")
			pf, err :=
				portforwarder.New().
					WithPod(fmt.Sprintf("%s-0", mariaDb.Name)).
					WithNamespace(mariaDb.Namespace).
					WithPorts(fmt.Sprint(mariaDb.Spec.Port)).
					WithOutputWriter(GinkgoWriter).
					WithErrorWriter(GinkgoWriter).
					Build()
			Expect(err).NotTo(HaveOccurred())

			go func() {
				if err := pf.Run(ctx); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			}()

			By("Creating a DatabaseMariaDB")
			databaseKey := types.NamespacedName{
				Name:      "data-test",
				Namespace: defaultNamespace,
			}
			database := databasev1alpha1.DatabaseMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      databaseKey.Name,
					Namespace: databaseKey.Namespace,
				},
				Spec: databasev1alpha1.DatabaseMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: mariaDb.Name,
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(ctx, &database)).To(Succeed())

			By("Expecting DatabaseMariaDB to be ready eventually")
			Eventually(func() bool {
				var database databasev1alpha1.DatabaseMariaDB
				if err := k8sClient.Get(ctx, databaseKey, &database); err != nil {
					return false
				}
				return database.IsReady()
			}, timeout, interval).Should(BeTrue())

			By("Deleting DatabaseMariaDB")
			Expect(k8sClient.Delete(ctx, &database)).To(Succeed())
		})
	})
})
