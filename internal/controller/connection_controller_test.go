package controller

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Connection", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	DescribeTable("should reconcile", func(conn *mariadbv1alpha1.Connection, wantDsn string) {
		key := client.ObjectKeyFromObject(conn)
		Expect(k8sClient.Create(testCtx, conn)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, conn)).To(Succeed())
		})

		By("Expecting Connection to be ready eventually")
		Eventually(func() bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Secret")
		var secret corev1.Secret
		Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())

		dsn, ok := secret.Data["dsn"]
		By("Expecting Secret key to be valid")
		Expect(ok).To(BeTrue())
		Expect(string(dsn)).To(Equal(wantDsn))
	},
		Entry(
			"Creating a Connection",
			&mariadbv1alpha1.Connection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conn-test",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.ConnectionSpec{
					ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string { t := "conn-test"; return &t }(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"foo": "bar",
								},
							},
							Key: func() *string { k := "dsn"; return &k }(),
						},
						HealthCheck: &mariadbv1alpha1.HealthCheck{
							Interval:      &metav1.Duration{Duration: 1 * time.Second},
							RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
						},
						Params: map[string]string{
							"parseTime": "true",
						},
					},
					MariaDBRef: &mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
				},
			},
			"test:MariaDB11!@tcp(mdb-test.default.svc.cluster.local:3306)/test?timeout=5s&parseTime=true",
		),
		Entry(
			"Creating a Connection providing ServiceName",
			&mariadbv1alpha1.Connection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conn-test-pod-0",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.ConnectionSpec{
					ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string { t := "conn-test-pod-0"; return &t }(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"foo": "bar",
								},
							},
							Key: func() *string { k := "dsn"; return &k }(),
						},
						HealthCheck: &mariadbv1alpha1.HealthCheck{
							Interval:      &metav1.Duration{Duration: 1 * time.Second},
							RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
						},
						Params: map[string]string{
							"parseTime": "true",
						},
						ServiceName: &testMdbkey.Name,
					},
					MariaDBRef: &mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
				},
			},
			"test:MariaDB11!@tcp(mdb-test.default.svc.cluster.local:3306)/test?timeout=5s&parseTime=true",
		),
		Entry(
			"Creating a Connection providing DSN Format",
			&mariadbv1alpha1.Connection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conn-test-custom-dsn",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.ConnectionSpec{
					ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string { t := "conn-test-custom-dsn"; return &t }(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"foo": "bar",
								},
							},
							Key: func() *string { k := "dsn"; return &k }(),
							Format: func() *string {
								f := "mysql://{{ .Username }}:{{ .Password }}@{{ .Host }}:{{ .Port }}/{{ .Database }}{{ .Params }}"
								return &f
							}(),
						},
						HealthCheck: &mariadbv1alpha1.HealthCheck{
							Interval:      &metav1.Duration{Duration: 1 * time.Second},
							RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
						},
						Params: map[string]string{
							"timeout": (5 * time.Second).String(),
						},
					},
					MariaDBRef: &mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
				},
			},
			"mysql://test:MariaDB11!@mdb-test.default.svc.cluster.local:3306/test?timeout=5s",
		),
	)

	It("should default", func() {
		key := types.NamespacedName{
			Name:      "conn-default-test",
			Namespace: testNamespace,
		}
		conn := mariadbv1alpha1.Connection{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.ConnectionSpec{
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				Username: testUser,
				PasswordSecretKeyRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				Database: &testDatabase,
			},
		}
		By("Creating Connection")
		Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &conn)).To(Succeed())
		})

		By("Expecting Connection to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting fields to have default values eventually")
		Expect(*conn.Spec.SecretName).To(Equal(key.Name))
		Expect(*conn.Spec.SecretTemplate.Key).To(Equal("dsn"))
	})

	It("should add extended information to Secret", func() {
		key := types.NamespacedName{
			Name:      "conn-default-extended-test",
			Namespace: testNamespace,
		}
		conn := mariadbv1alpha1.Connection{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.ConnectionSpec{
				ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
					SecretTemplate: &mariadbv1alpha1.SecretTemplate{
						UsernameKey: func() *string { k := "user"; return &k }(),
						PasswordKey: func() *string { k := "pass"; return &k }(),
						HostKey:     func() *string { k := "host"; return &k }(),
						PortKey:     func() *string { k := "port"; return &k }(),
						DatabaseKey: func() *string { k := "name"; return &k }(),
					},
				},
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				Username: testUser,
				PasswordSecretKeyRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				Database: &testDatabase,
			},
		}
		By("Creating Connection")
		Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &conn)).To(Succeed())
		})

		By("Expecting Connection to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Secret")
		var secret corev1.Secret
		Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())

		By("Expecting Secret key to contain extended information")
		user, ok := secret.Data["user"]
		Expect(ok).To(BeTrue())
		Expect(string(user)).To(Equal(testUser))
		pass, ok := secret.Data["pass"]
		Expect(ok).To(BeTrue())
		Expect(string(pass)).To(Equal("MariaDB11!"))
		host, ok := secret.Data["host"]
		Expect(ok).To(BeTrue())
		Expect(string(host)).To(Equal("mdb-test.default.svc.cluster.local"))
		port, ok := secret.Data["port"]
		Expect(ok).To(BeTrue())
		Expect(string(port)).To(Equal("3306"))
		database, ok := secret.Data["name"]
		Expect(ok).To(BeTrue())
		Expect(string(database)).To(Equal(testDatabase))
	})

	It("should update Secret", func() {
		key := types.NamespacedName{
			Name:      "conn-update-test",
			Namespace: testNamespace,
		}
		secretKey := "dsn"

		By("Creating Secret")
		passwordKey := types.NamespacedName{
			Name:      "conn-update-test-password",
			Namespace: testNamespace,
		}
		passwordSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      passwordKey.Name,
				Namespace: passwordKey.Namespace,
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			StringData: map[string]string{
				secretKey: "MariaDB11!",
			},
		}
		Expect(k8sClient.Create(testCtx, &passwordSecret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &passwordSecret)).To(Succeed())
		})

		conn := mariadbv1alpha1.Connection{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.ConnectionSpec{
				ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
					SecretName: ptr.To(key.Name),
					SecretTemplate: &mariadbv1alpha1.SecretTemplate{
						Key: ptr.To(secretKey),
					},
				},
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				Username: testUser,
				PasswordSecretKeyRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: passwordSecret.Name,
					},
					Key: secretKey,
				},
				Database: &testDatabase,
			},
		}
		By("Creating Connection")
		Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &conn)).To(Succeed())
		})

		By("Expecting Secret to be updated")
		Eventually(func(g Gomega) bool {
			var secret corev1.Secret
			if err := k8sClient.Get(testCtx, key, &secret); err != nil {
				return false
			}
			g.Expect(secret.Data[secretKey]).To(
				BeEquivalentTo("test:MariaDB11!@tcp(mdb-test.default.svc.cluster.local:3306)/test?timeout=5s"),
			)
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Updating Connection username")
		Eventually(func(g Gomega) bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
				return false
			}
			conn.Spec.Username = "updated-test"
			g.Expect(k8sClient.Update(testCtx, &conn)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Secret username to be updated")
		Eventually(func(g Gomega) bool {
			var secret corev1.Secret
			if err := k8sClient.Get(testCtx, key, &secret); err != nil {
				return false
			}
			g.Expect(secret.Data[secretKey]).To(
				BeEquivalentTo("updated-test:MariaDB11!@tcp(mdb-test.default.svc.cluster.local:3306)/test?timeout=5s"),
			)
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Updating Connection password")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, passwordKey, &passwordSecret); err != nil {
				return false
			}
			passwordSecret.Data[secretKey] = []byte("MariaDB-updated11!")
			g.Expect(k8sClient.Update(testCtx, &passwordSecret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Secret password to be updated")
		Eventually(func(g Gomega) bool {
			var secret corev1.Secret
			if err := k8sClient.Get(testCtx, key, &secret); err != nil {
				return false
			}
			g.Expect(secret.Data[secretKey]).To(
				BeEquivalentTo("updated-test:MariaDB-updated11!@tcp(mdb-test.default.svc.cluster.local:3306)/test?timeout=5s"),
			)
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
