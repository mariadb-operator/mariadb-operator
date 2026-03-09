package controller

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("External MariaDB spec", func() {
	It("should default", func() {

		By("Waiting for external emulated MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testEmulateExternalMdbkey)

		By("Creating External MariaDB")
		key := types.NamespacedName{
			Name:      "test-external-mariadb-default",
			Namespace: testNamespace,
		}
		emdb := mariadbv1alpha1.ExternalMariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.ExternalMariaDBSpec{
				Host:     testEmulateExternalMdbHost,
				Username: ptr.To("root"),
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testEmulatedExternalPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
					Annotations: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
				},
				Connection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: &key.Name,
					HealthCheck: &mariadbv1alpha1.HealthCheck{
						Interval:      &metav1.Duration{Duration: 1 * time.Second},
						RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
					},
				},
				TLS: &mariadbv1alpha1.ExternalTLS{
					TLS: mariadbv1alpha1.TLS{
						Enabled:  true,
						Required: ptr.To(false),
						ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mdb-emulate-external-test-ca",
						},
						ClientCertSecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mdb-emulate-external-test-client-cert",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &emdb)).To(Succeed())
		DeferCleanup(func() {
			deleteExternalMariadb(key, false)
		})

		By("Expecting to eventually default")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &emdb); err != nil {
				return false
			}
			return emdb.Spec.Port == 3306
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting IsTLSMutual to return true by default")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &emdb); err != nil {
				return false
			}
			return emdb.IsTLSMutual()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Connection to be ready eventually")
		Eventually(func(g Gomega) bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&emdb), &conn); err != nil {
				return false
			}
			g.Expect(conn.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(conn.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(conn.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(conn.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should handle mutual TLS disabled", func() {
		By("Creating External MariaDB with mutual TLS disabled")
		key := types.NamespacedName{
			Name:      "test-external-mariadb-no-mutual-tls",
			Namespace: testNamespace,
		}
		emdb := mariadbv1alpha1.ExternalMariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.ExternalMariaDBSpec{
				Host:     testEmulateExternalMdbHost,
				Username: ptr.To("root"),
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testEmulatedExternalPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				Connection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: &key.Name,
					HealthCheck: &mariadbv1alpha1.HealthCheck{
						Interval:      &metav1.Duration{Duration: 1 * time.Second},
						RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
					},
				},
				TLS: &mariadbv1alpha1.ExternalTLS{
					TLS: mariadbv1alpha1.TLS{
						Enabled:  true,
						Required: ptr.To(false),
						ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mdb-emulate-external-test-ca",
						},
					},
					Mutual: ptr.To(false),
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &emdb)).To(Succeed())
		DeferCleanup(func() {
			deleteExternalMariadb(key, false)
		})

		By("Expecting IsTLSMutual to return false")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &emdb); err != nil {
				return false
			}
			return !emdb.IsTLSMutual()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Connection to be ready eventually")
		Eventually(func(g Gomega) bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&emdb), &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should delete sql resources", func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testEmulateExternalMdbkey)

		By("Creating External MariaDB")
		emdbKey := types.NamespacedName{
			Name:      "test-external-mariadb-default",
			Namespace: testNamespace,
		}
		emdb := mariadbv1alpha1.ExternalMariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      emdbKey.Name,
				Namespace: emdbKey.Namespace,
			},
			Spec: mariadbv1alpha1.ExternalMariaDBSpec{
				Host:     testEmulateExternalMdbHost,
				Username: ptr.To("root"),
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testEmulatedExternalPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
					Annotations: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
				},
				TLS: &mariadbv1alpha1.ExternalTLS{
					TLS: mariadbv1alpha1.TLS{
						Enabled:  true,
						Required: ptr.To(false),
						ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mdb-emulate-external-test-ca",
						},
						ClientCertSecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mdb-emulate-external-test-client-cert",
						},
						ServerCertSecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mdb-emulate-external-test-server-cert",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &emdb)).To(Succeed())
		DeferCleanup(func() {
			deleteExternalMariadb(emdbKey, true)
		})

		By("Expecting ExternalMariaDB to be ready eventually")
		Eventually(func(g Gomega) bool {
			var mdb mariadbv1alpha1.ExternalMariaDB
			g.Expect(k8sClient.Get(testCtx, emdbKey, &mdb)).To(Succeed())
			return mdb.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating Database")
		dbKey := types.NamespacedName{
			Name:      "test-db-finalize",
			Namespace: testNamespace,
		}

		db := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dbKey.Name,
				Namespace: dbKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: emdbKey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				CharacterSet: "utf8",
				Collate:      "utf8_general_ci",
			},
		}
		Expect(k8sClient.Create(testCtx, &db)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, &db))).To(Succeed())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, dbKey, &db)).To(Succeed())
			return db.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating User")
		userKey := types.NamespacedName{
			Name:      "test-user-finalize",
			Namespace: testNamespace,
		}
		passwordSecretKey := types.NamespacedName{
			Name:      "test-user-password-finalize",
			Namespace: testNamespace,
		}
		passwordSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      passwordSecretKey.Name,
				Namespace: passwordSecretKey.Namespace,
			},
			StringData: map[string]string{
				"password": "test-password",
			},
		}
		Expect(k8sClient.Create(testCtx, &passwordSecret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &passwordSecret)).To(Succeed())
		})

		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: emdbKey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: passwordSecretKey.Name,
					},
					Key: "password",
				},
				MaxUserConnections: 20,
			},
		}
		Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, &user))).To(Succeed())
		})

		By("Expecting User to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, userKey, &user)).To(Succeed())
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Creating Grant")
		grantKey := types.NamespacedName{
			Name:      "test-grant-finalize",
			Namespace: testNamespace,
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantKey.Name,
				Namespace: grantKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: emdbKey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: true,
				},
				Privileges: []string{
					"SELECT",
					"INSERT",
					"UPDATE",
				},
				Database: dbKey.Name,
				Table:    "*",
				Username: userKey.Name,
			},
		}
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, &grant))).To(Succeed())
		})

		By("Expecting Grant to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, grantKey, &grant)).To(Succeed())
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Deleting User")
		Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())

		By("Deleting Grant")
		Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())

		By("Deleting Database")
		Expect(k8sClient.Delete(testCtx, &db)).To(Succeed())

		By("Expecting User to be deleted eventually")
		Eventually(func() bool {
			err := k8sClient.Get(testCtx, userKey, &mariadbv1alpha1.User{})
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to be deleted eventually")
		Eventually(func() bool {
			err := k8sClient.Get(testCtx, grantKey, &mariadbv1alpha1.Grant{})
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Database to be deleted eventually")
		Eventually(func() bool {
			err := k8sClient.Get(testCtx, dbKey, &mariadbv1alpha1.Database{})
			return apierrors.IsNotFound(err)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting User and Database to be deleted from MariaDB")
		var emulatedMdb mariadbv1alpha1.MariaDB
		Expect(k8sClient.Get(testCtx, testEmulateExternalMdbkey, &emulatedMdb)).To(Succeed())
		refResolver := refresolver.New(k8sClient)
		sqlClient, err := sql.NewClientWithMariaDB(testCtx, &emulatedMdb, refResolver)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			sqlClient.Close()
		})

		Eventually(func(g Gomega) {
			dbExists, err := sqlClient.Exists(testCtx, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?;", dbKey.Name)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(dbExists).To(BeFalse())
		}, testTimeout, testInterval).Should(Succeed())

		Eventually(func(g Gomega) {
			userExists, err := sqlClient.UserExists(testCtx, userKey.Name, "%")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(userExists).To(BeFalse())
		}, testTimeout, testInterval).Should(Succeed())
	})
})

func TestGetVersion(t *testing.T) {
	r := &ExternalMariaDBReconciler{}

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"simple patch", "11.4.4-MariaDB", "11.4", false},
		{"build version", "11.4.7-4.2", "11.4", false},
		{"empty string", "", "", true},
		{"no dot", "10", "", true},
		{"nonsense", "bad-version", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.getVersion(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getVersion(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("getVersion(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
