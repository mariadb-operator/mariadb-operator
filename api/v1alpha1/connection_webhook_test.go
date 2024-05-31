package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			func(conn *Connection, wantErr bool) {
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
				&Connection{
					ObjectMeta: meta,
					Spec: ConnectionSpec{
						Username: "foo",
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				true,
			),
			Entry(
				"MariaDB ref",
				&Connection{
					ObjectMeta: meta,
					Spec: ConnectionSpec{
						MariaDBRef: &MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				false,
			),
			Entry(
				"MaxScale ref",
				&Connection{
					ObjectMeta: meta,
					Spec: ConnectionSpec{
						MaxScaleRef: &corev1.ObjectReference{
							Name: "foo",
						},
						Username: "foo",
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "foo",
							},
						},
					},
				},
				false,
			),
			Entry(
				"MariaDB and MaxScale refs",
				&Connection{
					ObjectMeta: meta,
					Spec: ConnectionSpec{
						MaxScaleRef: &corev1.ObjectReference{
							Name: "foo",
						},
						MariaDBRef: &MariaDBRef{
							ObjectReference: corev1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
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
			conn := Connection{
				ObjectMeta: v1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: ConnectionSpec{
					ConnectionTemplate: ConnectionTemplate{
						SecretName: func() *string { t := "test"; return &t }(),
						SecretTemplate: &SecretTemplate{
							Metadata: &Metadata{
								Labels: map[string]string{
									"foo": "bar",
								},
							},
						},
						HealthCheck: &HealthCheck{
							Interval:      &metav1.Duration{Duration: 1 * time.Second},
							RetryInterval: &metav1.Duration{Duration: 1 * time.Second},
						},
					},
					MariaDBRef: &MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					Username: "test",
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
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
			func(patchFn func(conn *Connection), wantErr bool) {
				var conn Connection
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
				func(conn *Connection) {
					conn.Spec.MariaDBRef.Name = "foo"
				},
				false,
			),
			Entry(
				"Updating Username",
				func(conn *Connection) {
					conn.Spec.Username = "foo"
				},
				false,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(conn *Connection) {
					conn.Spec.PasswordSecretKeyRef.Key = "foo"
				},
				false,
			),
			Entry(
				"Updating Database",
				func(conn *Connection) {
					conn.Spec.Database = func() *string { t := "foo"; return &t }()
				},
				false,
			),
			Entry(
				"Updating SecretName",
				func(conn *Connection) {
					conn.Spec.SecretName = func() *string { s := "foo"; return &s }()
				},
				true,
			),
			Entry(
				"Updating SecretTemplate",
				func(conn *Connection) {
					conn.Spec.SecretTemplate.Metadata.Labels = map[string]string{
						"foo": "foo",
					}
				},
				false,
			),
			Entry(
				"Updating HealthCheck",
				func(conn *Connection) {
					conn.Spec.HealthCheck.Interval = &metav1.Duration{Duration: 3 * time.Second}
					conn.Spec.HealthCheck.RetryInterval = &metav1.Duration{Duration: 3 * time.Second}
				},
				false,
			),
		)
	})
})
