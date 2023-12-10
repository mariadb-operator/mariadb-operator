package controller

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Connection controller", func() {
	Context("When creating a Connection", func() {
		DescribeTable(
			"Should reconcile",
			func(conn *mariadbv1alpha1.Connection, wantDsn string) {
				key := client.ObjectKeyFromObject(conn)
				Expect(k8sClient.Create(testCtx, conn)).To(Succeed())

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

				By("Deleting Connection")
				Expect(k8sClient.Delete(testCtx, conn)).To(Succeed())
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
								Labels: map[string]string{
									"foo": "bar",
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
						MariaDBRef: mariadbv1alpha1.MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: testMariaDbName,
							},
							WaitForIt: true,
						},
						Username: testUser,
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: testPwdSecretName,
							},
							Key: testPwdSecretKey,
						},
						Database: &testDatabase,
					},
				},
				"test:test@tcp(mariadb-test.default.svc.cluster.local:3306)/test?parseTime=true",
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
								Labels: map[string]string{
									"foo": "bar",
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
							ServiceName: &testMariaDbName,
						},
						MariaDBRef: mariadbv1alpha1.MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: testMariaDbName,
							},
							WaitForIt: true,
						},
						Username: testUser,
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: testPwdSecretName,
							},
							Key: testPwdSecretKey,
						},
						Database: &testDatabase,
					},
				},
				"test:test@tcp(mariadb-test.default.svc.cluster.local:3306)/test?parseTime=true",
			),
			Entry(
				"Creating a Connection providing DSN Format",
				&mariadbv1alpha1.Connection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "conn-test",
						Namespace: testNamespace,
					},
					Spec: mariadbv1alpha1.ConnectionSpec{
						ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
							SecretName: func() *string { t := "conn-test"; return &t }(),
							SecretTemplate: &mariadbv1alpha1.SecretTemplate{
								Labels: map[string]string{
									"foo": "bar",
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
								"parseTime": "true",
							},
						},
						MariaDBRef: mariadbv1alpha1.MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: testMariaDbName,
							},
							WaitForIt: true,
						},
						Username: testUser,
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: testPwdSecretName,
							},
							Key: testPwdSecretKey,
						},
						Database: &testDatabase,
					},
				},
				"mysql://test:test@mariadb-test.default.svc.cluster.local:3306/test?parseTime=true",
			),
		)

		It("Should default", func() {
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
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdSecretName,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
				},
			}
			By("Creating Connection")
			Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())

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

			By("Deleting Connection")
			Expect(k8sClient.Delete(testCtx, &conn)).To(Succeed())
		})

		It("Should add extended information to secret", func() {
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
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdSecretName,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
				},
			}
			By("Creating Connection")
			Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())

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
			Expect(string(pass)).To(Equal("test"))
			host, ok := secret.Data["host"]
			Expect(ok).To(BeTrue())
			Expect(string(host)).To(Equal("mariadb-test.default.svc.cluster.local"))
			port, ok := secret.Data["port"]
			Expect(ok).To(BeTrue())
			Expect(string(port)).To(Equal("3306"))
			database, ok := secret.Data["name"]
			Expect(ok).To(BeTrue())
			Expect(string(database)).To(Equal(testDatabase))

			By("Deleting Connection")
			Expect(k8sClient.Delete(testCtx, &conn)).To(Succeed())
		})
	})
})
