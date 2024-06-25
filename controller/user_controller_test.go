package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("User", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should reconcile", func() {
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
				PasswordSecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				MaxUserConnections: 20,
			},
		}
		Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())
		})

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
	})

	It("should update password", func() {
		key := types.NamespacedName{
			Name:      "user-password-update",
			Namespace: testNamespace,
		}
		secretKey := "password"

		By("Creating Secret")
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			StringData: map[string]string{
				secretKey: "MariaDB11!",
			},
		}
		Expect(k8sClient.Create(testCtx, &secret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &secret)).To(Succeed())
		})

		By("Creating User")
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: key.Name,
					},
					Key: secretKey,
				},
				MaxUserConnections: 20,
			},
		}
		Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())
		})

		By("Expecting User to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &user); err != nil {
				return false
			}
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid")
		testValidCredentials(user.Name, *user.Spec.PasswordSecretKeyRef)

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())
			secret.Data[secretKey] = []byte("MariaDB12!")
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid")
		testValidCredentials(user.Name, *user.Spec.PasswordSecretKeyRef)
	})
})
