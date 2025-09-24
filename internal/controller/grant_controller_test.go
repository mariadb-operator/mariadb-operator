package controller

import (
	"fmt"
	"strconv"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Grant", Label("basic"), func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should grant privileges for all tables and databases", func() {
		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-all-test",
			Namespace: testNamespace,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-select-insert-update-test",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
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
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, grantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should grant privileges for a database", func() {
		By("Creating a Database")
		databaseKey := types.NamespacedName{
			Name:      "grant-database-test",
			Namespace: testNamespace,
		}
		database := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      databaseKey.Name,
				Namespace: databaseKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				CharacterSet: "utf8",
				Collate:      "utf8_general_ci",
			},
		}
		Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
				return false
			}
			return database.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-database-test",
			Namespace: testNamespace,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-all-test",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				Privileges: []string{
					"ALL",
				},
				Database:    testDatabase,
				Table:       "*",
				Username:    userKey.Name,
				GrantOption: true,
			},
		}
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, grantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})
})

var _ = Describe("Grant on an external MariaDB", func() {
	BeforeEach(func() {
		By("Waiting for External MariaDB to be ready")
		expectExternalMariadbReady(testCtx, k8sClient, testEMdbkey)
	})

	It("should grant privileges for all tables and databases", func() {
		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-all-test",
			Namespace: testNamespace,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testEMdbkey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-select-insert-update-test",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testEMdbkey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
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
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, grantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should grant privileges for a database", func() {
		By("Creating a Database")
		databaseKey := types.NamespacedName{
			Name:      "grant-database-test",
			Namespace: testNamespace,
		}
		database := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      databaseKey.Name,
				Namespace: databaseKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testEMdbkey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				CharacterSet: "utf8",
				Collate:      "utf8_general_ci",
			},
		}
		Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, databaseKey, &database); err != nil {
				return false
			}
			return database.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-database-test",
			Namespace: testNamespace,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testEMdbkey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-all-test",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testEMdbkey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				Privileges: []string{
					"ALL",
				},
				Database:    testDatabase,
				Table:       "*",
				Username:    userKey.Name,
				GrantOption: true,
			},
		}
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, grantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})
})

var _ = Describe("Grant on a MariaDB replicating from external MariaDB", func() {
	var (
		// 	key = testMdbERkey
		mdb = &mariadbv1alpha1.MariaDB{}
	)

	It("should grant privileges for all tables and databases", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, testMdbERkey, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice

		By("Expecting to create secondary Endpoints: " + strconv.Itoa(int(mdb.Spec.Replicas)))
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == int(mdb.Spec.Replicas)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating a User")
		userKey := types.NamespacedName{
			Name:      "grant-user-all-erep-test",
			Namespace: testNamespace,
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbERkey.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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

		By("Creating a Grant")
		grantKey := types.NamespacedName{
			Name:      "grant-select-insert-update-erep-test",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				SQLTemplate: mariadbv1alpha1.SQLTemplate{
					RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbERkey.Name,
					},
					WaitForIt: true,
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
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, grantKey, &grant); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&grant, grantFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())

		replicas := int(mdb.Spec.Replicas)
		refResolver := refresolver.New(k8sClient)

		By("Expecting to get password from secret")
		password, err := refResolver.SecretKeyRef(testCtx, testPasswordSecretRef, mdb.GetNamespace())
		Expect(err).To(Succeed())

		for i := 0; i < replicas; i++ {

			client, err := sql.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i,
				sql.WithUsername(user.Name),
				sql.WithPassword(password))

			By("Expecting to get SqlClient from Pod " + strconv.Itoa(i))
			Expect(err).To(Succeed())

			By("Expecting to GRANT exists on Pod" + strconv.Itoa(i) + " eventually")
			Eventually(func() bool {
				exists, err := client.GrantExists(testCtx, grant.Spec.Privileges, grant.Spec.Database, grant.Spec.Table, grant.AccountName())
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error: %v\n", err)
					return false
				}
				return exists
			}, testTimeout, testInterval).Should(BeTrue())

			Expect(client.Exec(testCtx, "SELECT 1")).To(Succeed())
		}
	})

})
