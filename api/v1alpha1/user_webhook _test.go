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

var _ = Describe("User webhook", func() {
	Context("When updating a User", func() {
		It("Should validate", func() {
			By("Creating User")
			key := types.NamespacedName{
				Name:      "user-mariadb-webhook",
				Namespace: testNamespace,
			}
			initialUser := User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: UserSpec{
					MariaDBRef: MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "user-mariadb-webhook-root",
						},
						Key: "password",
					},
					MaxUserConnections: 10,
				},
			}
			Expect(k8sClient.Create(testCtx, &initialUser)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *User)
				wantErr bool
			}{
				{
					by: "Updating MariaDBRef",
					patchFn: func(umdb *User) {
						umdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating PasswordSecretKeyRef",
					patchFn: func(umdb *User) {
						umdb.Spec.PasswordSecretKeyRef.Name = "another-secret"
					},
					wantErr: true,
				},
				{
					by: "Updating MaxUserConnections",
					patchFn: func(umdb *User) {
						umdb.Spec.MaxUserConnections = 20
					},
					wantErr: true,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &initialUser)).To(Succeed())

				patch := client.MergeFrom(initialUser.DeepCopy())
				t.patchFn(&initialUser)

				err := k8sClient.Patch(testCtx, &initialUser, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
