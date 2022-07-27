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

var _ = Describe("GrantMariaDB controller", func() {
	Context("When creating a GrantMariaDB", func() {
		It("Should reconcile", func() {
			By("Creating a UserMariaDB")
			userKey := types.NamespacedName{
				Name:      "grant-user-test",
				Namespace: defaultNamespace,
			}
			user := databasev1alpha1.UserMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userKey.Name,
					Namespace: userKey.Namespace,
				},
				Spec: databasev1alpha1.UserMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: mariaDbKey.Name,
					},
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mariaDbRootPwdKey.Name,
						},
						Key: mariaDbRootPwdSecretKey,
					},
					MaxUserConnections: 20,
				},
			}
			Expect(k8sClient.Create(ctx, &user)).To(Succeed())

			By("Expecting UserMariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, userKey, &user); err != nil {
					return false
				}
				return user.IsReady()
			}, timeout, interval).Should(BeTrue())

			By("Creating a GrantMariaDB")
			grantKey := types.NamespacedName{
				Name:      "grant-test",
				Namespace: defaultNamespace,
			}
			grant := databasev1alpha1.GrantMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      grantKey.Name,
					Namespace: grantKey.Namespace,
				},
				Spec: databasev1alpha1.GrantMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: mariaDbKey.Name,
					},
					Privileges: []string{
						"SELECT",
						"INSERT",
						"UPDATE",
					},
					Database:    "*",
					Table:       "*",
					Username:    userKey.Name,
					GrantOption: true,
				},
			}
			Expect(k8sClient.Create(ctx, &grant)).To(Succeed())

			By("Expecting GrantMariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, grantKey, &grant); err != nil {
					return false
				}
				return grant.IsReady()
			}, timeout, interval).Should(BeTrue())

			By("Deleting UserMariaDB")
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())

			By("Deleting GrantMariaDB")
			Expect(k8sClient.Delete(ctx, &grant)).To(Succeed())
		})
	})
})
