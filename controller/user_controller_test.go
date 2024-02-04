package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
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
							Name: testMdbkey.Name,
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
