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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Database webhook", func() {
	Context("When updating a Database", func() {
		It("Should validate", func() {
			By("Creating Database")
			key := types.NamespacedName{
				Name:      "database-mariadb-webhook",
				Namespace: testNamespace,
			}
			database := Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: DatabaseSpec{
					MariaDBRef: MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(testCtx, &database)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mmontes11/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *Database)
				wantErr bool
			}{
				{
					by: "Updating MariaDBRef",
					patchFn: func(dmdb *Database) {
						dmdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating CharacterSet",
					patchFn: func(dmdb *Database) {
						dmdb.Spec.CharacterSet = "utf16"
					},
					wantErr: true,
				},
				{
					by: "Updating Collate",
					patchFn: func(dmdb *Database) {
						dmdb.Spec.Collate = "latin2_general_ci"
					},
					wantErr: true,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &database)).To(Succeed())

				patch := client.MergeFrom(database.DeepCopy())
				t.patchFn(&database)

				err := k8sClient.Patch(testCtx, &database, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
