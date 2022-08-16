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

var _ = Describe("GrantMariaDB webhook", func() {
	Context("When updating a GrantMariaDB", func() {
		FIt("Should validate", func() {
			By("Creating GrantMariaDB")
			key := types.NamespacedName{
				Name:      "grant-mariadb-webhook",
				Namespace: testNamespace,
			}
			initialGrant := GrantMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: GrantMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: "mariadb-webhook",
					},
					Privileges: []string{
						"SELECT",
					},
					Database:    "foo",
					Table:       "foo",
					Username:    "foo",
					GrantOption: false,
				},
			}
			Expect(k8sClient.Create(testCtx, &initialGrant)).To(Succeed())

			By("Updating MariaDBRef")
			grant := initialGrant.DeepCopy()
			grant.Spec.MariaDBRef = corev1.LocalObjectReference{
				Name: "another-mariadb",
			}
			err := k8sClient.Update(testCtx, grant)
			Expect(err).To(HaveOccurred())

			By("Updating Privileges")
			grant = initialGrant.DeepCopy()
			grant.Spec.Privileges = []string{
				"SELECT",
				"UPDATE",
			}
			err = k8sClient.Update(testCtx, grant)
			Expect(err).To(HaveOccurred())

			By("Updating Database")
			grant = initialGrant.DeepCopy()
			grant.Spec.Database = "bar"
			err = k8sClient.Update(testCtx, grant)
			Expect(err).To(HaveOccurred())

			By("Updating Table")
			grant = initialGrant.DeepCopy()
			grant.Spec.Table = "bar"
			err = k8sClient.Update(testCtx, grant)
			Expect(err).To(HaveOccurred())

			By("Updating Username")
			grant = initialGrant.DeepCopy()
			grant.Spec.Username = "bar"
			err = k8sClient.Update(testCtx, grant)
			Expect(err).To(HaveOccurred())

			By("Updating GrantOption")
			grant = initialGrant.DeepCopy()
			grant.Spec.GrantOption = true
			err = k8sClient.Update(testCtx, grant)
			Expect(err).To(HaveOccurred())
		})
	})
})
