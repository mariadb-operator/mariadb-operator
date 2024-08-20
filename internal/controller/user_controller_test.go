package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
		testConnection(user.Name, *user.Spec.PasswordSecretKeyRef, testDatabase, true)

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())
			secret.Data[secretKey] = []byte("MariaDB12!")
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid")
		testConnection(user.Name, *user.Spec.PasswordSecretKeyRef, testDatabase, true)
	})

	It("should update password hash", func() {
		key := types.NamespacedName{
			Name:      "user-password-hash-update",
			Namespace: testNamespace,
		}
		secretKeyPassword := "password"
		secretKeyHash := "passwordHash"

		PasswordSecretKeyRef := &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: key.Name,
			},
			Key: secretKeyPassword,
		}

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
				secretKeyPassword: "MariaDB11!",
				secretKeyHash:     "*57685B4F0FF9D049082E296E2C39354B7A98774E",
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
				PasswordHashSecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: key.Name,
					},
					Key: secretKeyHash,
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
		testConnection(user.Name, *PasswordSecretKeyRef, testDatabase, true)

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())
			secret.Data[secretKeyPassword] = []byte("MariaDB12!")
			secret.Data[secretKeyHash] = []byte("*2951D147E3B9212E57872D4D958C44F4AE9CF0B0")
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid")
		testConnection(user.Name, *PasswordSecretKeyRef, testDatabase, true)
	})

	It("should update password plugin", func() {
		key := types.NamespacedName{
			Name:      "user-password-plugin-update",
			Namespace: testNamespace,
		}
		secretKeyPassword := "password"
		secretKeyPluginName := "pluginName"
		secretKeyPluginArg := "pluginArg"

		PasswordSecretKeyRef := &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: key.Name,
			},
			Key: secretKeyPassword,
		}

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
				secretKeyPassword:   "MariaDB11!",
				secretKeyPluginName: "mysql_native_password",
				secretKeyPluginArg:  "*57685B4F0FF9D049082E296E2C39354B7A98774E",
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
				PasswordPlugin: mariadbv1alpha1.PasswordPlugin{
					PluginNameSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: key.Name,
						},
						Key: secretKeyPluginName,
					},
					PluginArgSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: key.Name,
						},
						Key: secretKeyPluginArg,
					},
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
		testConnection(user.Name, *PasswordSecretKeyRef, testDatabase, true)

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())
			secret.Data[secretKeyPassword] = []byte("MariaDB12!")
			secret.Data[secretKeyPluginArg] = []byte("*2951D147E3B9212E57872D4D958C44F4AE9CF0B0")
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid")
		testConnection(user.Name, *PasswordSecretKeyRef, testDatabase, true)
	})

	It("should clean up", func() {
		By("Creating User")
		userKey := types.NamespacedName{
			Name:      "test-clean-up-user",
			Namespace: testNamespace,
		}
		passwordSecretKeyRef := corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: testPwdKey.Name,
			},
			Key: testPwdSecretKey,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicyDelete),
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &passwordSecretKeyRef,
				MaxUserConnections:   20,
			},
		}
		Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(removeFinalizerAndDelete(&user)).To(Succeed())
		})

		By("Creating Database")
		databaseKey := types.NamespacedName{
			Name:      "test-clean-up-database",
			Namespace: testNamespace,
		}
		database := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      databaseKey.Name,
				Namespace: databaseKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicyDelete),
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		DeferCleanup(func() {
			Expect(removeFinalizerAndDelete(&database)).To(Succeed())
		})

		By("Creating Grant")
		grantKey := types.NamespacedName{
			Name:      "test-clean-up-grant",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicyDelete),
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				Privileges: []string{
					"ALL PRIVILEGES",
				},
				Database: databaseKey.Name,
				Table:    "*",
				Username: userKey.Name,
				Host:     ptr.To("%"),
			},
		}
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(removeFinalizerAndDelete(&grant)).To(Succeed())
		})

		By("Expecting credentials to be valid")
		testConnection(userKey.Name, passwordSecretKeyRef, databaseKey.Name, true)

		By("Deleting Grant")
		Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		expectToNotExist(testCtx, k8sClient, &grant)

		By("Deleting Database")
		Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		expectToNotExist(testCtx, k8sClient, &database)

		By("Deleting User")
		Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())
		expectToNotExist(testCtx, k8sClient, &user)

		By("Expecting credentials to be invalid")
		testConnection(userKey.Name, passwordSecretKeyRef, databaseKey.Name, false)
	})

	It("should skip clean up", func() {
		By("Creating User")
		userKey := types.NamespacedName{
			Name:      "test-skip-clean-up-user",
			Namespace: testNamespace,
		}
		passwordSecretKeyRef := corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: testPwdKey.Name,
			},
			Key: testPwdSecretKey,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &passwordSecretKeyRef,
				MaxUserConnections:   20,
			},
		}
		Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(removeFinalizerAndDelete(&user)).To(Succeed())
		})

		By("Creating Database")
		databaseKey := types.NamespacedName{
			Name:      "test-skip-clean-up-database",
			Namespace: testNamespace,
		}
		database := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      databaseKey.Name,
				Namespace: databaseKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		DeferCleanup(func() {
			Expect(removeFinalizerAndDelete(&database)).To(Succeed())
		})

		By("Creating Grant")
		grantKey := types.NamespacedName{
			Name:      "test-skip-clean-up-grant",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					CleanupPolicy: ptr.To(mariadbv1alpha1.CleanupPolicySkip),
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				Privileges: []string{
					"ALL PRIVILEGES",
				},
				Database: databaseKey.Name,
				Table:    "*",
				Username: userKey.Name,
				Host:     ptr.To("%"),
			},
		}
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(removeFinalizerAndDelete(&grant)).To(Succeed())
		})

		By("Expecting credentials to be valid")
		testConnection(userKey.Name, passwordSecretKeyRef, databaseKey.Name, true)

		By("Deleting Grant")
		Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		expectToNotExist(testCtx, k8sClient, &grant)

		By("Deleting Database")
		Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		expectToNotExist(testCtx, k8sClient, &database)

		By("Deleting User")
		Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())
		expectToNotExist(testCtx, k8sClient, &user)

		By("Expecting credentials to be valid")
		testConnection(userKey.Name, passwordSecretKeyRef, databaseKey.Name, true)
	})
})
