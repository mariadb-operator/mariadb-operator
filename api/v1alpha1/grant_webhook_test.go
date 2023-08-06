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

var _ = Describe("Grant webhook", func() {
	Context("When updating a Grant", func() {
		It("Should validate", func() {
			By("Creating Grant")
			key := types.NamespacedName{
				Name:      "grant-mariadb-webhook",
				Namespace: testNamespace,
			}
			grant := Grant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: GrantSpec{
					MariaDBRef: MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
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
			Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *Grant)
				wantErr bool
			}{
				{
					by: "Updating MariaDBRef",
					patchFn: func(gmdb *Grant) {
						gmdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating Privileges",
					patchFn: func(gmdb *Grant) {
						gmdb.Spec.Privileges = []string{
							"SELECT",
							"UPDATE",
						}
					},
					wantErr: true,
				},
				{
					by: "Updating Database",
					patchFn: func(gmdb *Grant) {
						gmdb.Spec.Database = "bar"
					},
					wantErr: true,
				},
				{
					by: "Updating Table",
					patchFn: func(gmdb *Grant) {
						gmdb.Spec.Table = "bar"
					},
					wantErr: true,
				},
				{
					by: "Updating Username",
					patchFn: func(gmdb *Grant) {
						gmdb.Spec.Username = "bar"
					},
					wantErr: true,
				},
				{
					by: "Updating GrantOption",
					patchFn: func(gmdb *Grant) {
						gmdb.Spec.GrantOption = true
					},
					wantErr: true,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &grant)).To(Succeed())

				patch := client.MergeFrom(grant.DeepCopy())
				t.patchFn(&grant)

				err := k8sClient.Patch(testCtx, &grant, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
