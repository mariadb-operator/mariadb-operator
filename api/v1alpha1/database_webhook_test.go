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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Database webhook", func() {
	Context("When updating a Database", Ordered, func() {
		key := types.NamespacedName{
			Name:      "database-mariadb-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			database := Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: DatabaseSpec{
					MariaDBRef: MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					CharacterSet: "utf8",
					Collate:      "utf8_general_ci",
				},
			}
			Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		})

		DescribeTable(
			"Should validate",
			func(patchFn func(db *Database), wantErr bool) {
				var db Database
				Expect(k8sClient.Get(testCtx, key, &db)).To(Succeed())

				patch := client.MergeFrom(db.DeepCopy())
				patchFn(&db)

				err := k8sClient.Patch(testCtx, &db, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating MariaDBRef",
				func(db *Database) {
					db.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating CharacterSet",
				func(db *Database) {
					db.Spec.CharacterSet = "utf16"
				},
				true,
			),
			Entry(
				"Updating Collate",
				func(db *Database) {
					db.Spec.Collate = "latin2_general_ci"
				},
				true,
			),
		)
	})
})
