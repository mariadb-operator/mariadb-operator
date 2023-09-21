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

var _ = Describe("User controller", func() {
	Context("When creating a User", func() {
		It("Should reconcile", func() {
			userKey := types.NamespacedName{
				Name:      "user-test",
				Namespace: testNamespace,
			}
			user := mariadbv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userKey.Name,
					Namespace: userKey.Namespace,
				},
				Spec: mariadbv1alpha1.UserSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbKey.Name,
						},
						WaitForIt: true,
					},
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					MaxUserConnections: 20,
				},
			}
			Expect(k8sClient.Create(testCtx, &user)).To(Succeed())

			By("Expecting User to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, userKey, &user); err != nil {
					return false
				}
				return user.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting User to eventually have finalizer")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, userKey, &user); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(&user, userFinalizerName)
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting User")
			Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())
		})
	})
})
