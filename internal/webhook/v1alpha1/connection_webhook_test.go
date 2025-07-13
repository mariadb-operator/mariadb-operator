package v1alpha1

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Connection webhook", func() {
	Context("When creating a Connection", func() {
		meta := metav1.ObjectMeta{
			Name:      "connection-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(conn *mariadbv1alpha1.Connection, wantErr bool) {
				_ = k8sClient.Delete(testCtx, conn)
				err := k8sClient.Create(testCtx, conn)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No refs",
				&mariadbv1alpha1.Connection{
					ObjectMeta: meta,
					Spec: mariadbv1alpha1.ConnectionSpec{
						Username: "foo",
						PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				true,
			),
			Entry(
				"No creds",
				&mariadbv1alpha1.Connection{
					ObjectMeta: meta,
					Spec: mariadbv1alpha1.ConnectionSpec{
						MariaDBRef: &mariadbv1alpha1.MariaDBRef{
							ObjectReference: mariadbv1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
					},
				},
				true,
			),
			Entry(
				"TLS creds",
				&mariadbv1alpha1.Connection{
					ObjectMeta: meta,
					Spec: mariadbv1alpha1.ConnectionSpec{
						MariaDBRef: &mariadbv1alpha1.MariaDBRef{
							ObjectReference: mariadbv1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						TLSClientCertSecretRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "mariadb-client-tls",
						},
					},
				},
				false,
			),
			Entry(
				"MariaDB ref",
				&mariadbv1alpha1.Connection{
					ObjectMeta: meta,
					Spec: mariadbv1alpha1.ConnectionSpec{
						MariaDBRef: &mariadbv1alpha1.MariaDBRef{
							ObjectReference: mariadbv1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				false,
			),
			Entry(
				"MaxScale ref",
				&mariadbv1alpha1.Connection{
					ObjectMeta: meta,
					Spec: mariadbv1alpha1.ConnectionSpec{
						MaxScaleRef: &mariadbv1alpha1.ObjectReference{
							Name: "foo",
						},
						Username: "foo",
						PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				false,
			),
			Entry(
				"MariaDB and MaxScale refs",
				&mariadbv1alpha1.Connection{
					ObjectMeta: meta,
					Spec: mariadbv1alpha1.ConnectionSpec{
						MaxScaleRef: &mariadbv1alpha1.ObjectReference{
							Name: "foo",
						},
						MariaDBRef: &mariadbv1alpha1.MariaDBRef{
							ObjectReference: mariadbv1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				true,
			),
		)
	})
	Context("When updating a Connection", Ordered, func() {
		key := types.NamespacedName{
			Name:      "conn-update",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			conn := mariadbv1alpha1.Connection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.ConnectionSpec{
					ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string { t := "test"; return &t }(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"foo": "bar",
								},
							},
						},
						HealthCheck: &mariadbv1alpha1.HealthCheck{
							Interval:      &metav1.Duration{Duration: 1 * time.Second},
							RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
						},
					},
					MariaDBRef: &mariadbv1alpha1.MariaDBRef{
						ObjectReference: mariadbv1alpha1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					Username: "test",
					PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "test",
						},
						Key: "dsn",
					},
					Database: func() *string { t := "test"; return &t }(),
				},
			}
			Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())
		})

		DescribeTable(
			"Should validate",
			func(patchFn func(conn *mariadbv1alpha1.Connection), wantErr bool) {
				var conn mariadbv1alpha1.Connection
				Expect(k8sClient.Get(testCtx, key, &conn)).To(Succeed())

				patch := client.MergeFrom(conn.DeepCopy())
				patchFn(&conn)

				err := k8sClient.Patch(testCtx, &conn, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating MariaDBRef",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.MariaDBRef.Name = "foo"
				},
				false,
			),
			Entry(
				"Updating Username",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.Username = "foo"
				},
				false,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.PasswordSecretKeyRef.Key = "foo"
				},
				false,
			),
			Entry(
				"Updating Database",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.Database = func() *string { t := "foo"; return &t }()
				},
				false,
			),
			Entry(
				"Updating SecretName",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.SecretName = func() *string { s := "foo"; return &s }()
				},
				true,
			),
			Entry(
				"Updating SecretTemplate",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.SecretTemplate.Metadata.Labels = map[string]string{
						"foo": "foo",
					}
				},
				false,
			),
			Entry(
				"Updating HealthCheck",
				func(conn *mariadbv1alpha1.Connection) {
					conn.Spec.HealthCheck.Interval = &metav1.Duration{Duration: 3 * time.Second}
					conn.Spec.HealthCheck.RetryInterval = &metav1.Duration{Duration: 3 * time.Second}
				},
				false,
			),
		)
	})
})
