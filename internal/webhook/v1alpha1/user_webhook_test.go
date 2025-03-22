package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("v1alpha1.User webhook", func() {
	Context("When creating a v1alpha1.User", func() {
		DescribeTable(
			"Should validate",
			func(user *v1alpha1.User, wantErr bool) {
				err := k8sClient.Create(testCtx, user)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Valid cleanupPolicy",
				&v1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-user-valid-cleanuppolicy",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.UserSpec{
						SQLTemplate: v1alpha1.SQLTemplate{
							CleanupPolicy: ptr.To(v1alpha1.CleanupPolicyDelete),
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						PasswordSecretKeyRef: &v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "user-mariadb-webhook-root",
							},
							Key: "password",
						},
						MaxUserConnections: 10,
					},
				},
				false,
			),
			Entry(
				"Invalid cleanupPolicy",
				&v1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-user-invalid-cleanuppolicy",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.UserSpec{
						SQLTemplate: v1alpha1.SQLTemplate{
							CleanupPolicy: ptr.To(v1alpha1.CleanupPolicy("")),
						},
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						PasswordSecretKeyRef: &v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "user-mariadb-webhook-root",
							},
							Key: "password",
						},
						MaxUserConnections: 10,
					},
				},
				true,
			),
			Entry(
				"Valid require",
				&v1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-user-valid-require",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.UserSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						PasswordSecretKeyRef: &v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "user-mariadb-webhook-root",
							},
							Key: "password",
						},
						Require: &v1alpha1.TLSRequirements{
							Issuer:  ptr.To("/CN=mariadb-galera-ca"),
							Subject: ptr.To("/CN=mariadb-galera-ca"),
						},
						MaxUserConnections: 10,
					},
				},
				false,
			),
			Entry(
				"Invalid require",
				&v1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-user-invalid-require",
						Namespace: testNamespace,
					},
					Spec: v1alpha1.UserSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						PasswordSecretKeyRef: &v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "user-mariadb-webhook-root",
							},
							Key: "password",
						},
						Require: &v1alpha1.TLSRequirements{
							X509:    ptr.To(true),
							Issuer:  ptr.To("/CN=mariadb-galera-ca"),
							Subject: ptr.To("/CN=mariadb-galera-ca"),
						},
						MaxUserConnections: 10,
					},
				},
				true,
			),
		)
	})

	Context("When updating a v1alpha1.User", Ordered, func() {
		key := types.NamespacedName{
			Name:      "user-update-webhook",
			Namespace: testNamespace,
		}
		PasswordPlugin := v1alpha1.PasswordPlugin{
			PluginNameSecretKeyRef: &v1alpha1.SecretKeySelector{
				LocalObjectReference: v1alpha1.LocalObjectReference{
					Name: "user-mariadb-webhook-root",
				},
				Key: "pluginName",
			},
			PluginArgSecretKeyRef: &v1alpha1.SecretKeySelector{
				LocalObjectReference: v1alpha1.LocalObjectReference{
					Name: "user-mariadb-webhook-root",
				},
				Key: "pluginArg",
			},
		}
		BeforeAll(func() {
			user := v1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.UserSpec{
					MariaDBRef: v1alpha1.MariaDBRef{
						ObjectReference: v1alpha1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					PasswordSecretKeyRef: &v1alpha1.SecretKeySelector{
						LocalObjectReference: v1alpha1.LocalObjectReference{
							Name: "user-mariadb-webhook-root",
						},
						Key: "password",
					},
					MaxUserConnections: 10,
				},
			}
			Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(u *v1alpha1.User), wantErr bool) {
				var user v1alpha1.User
				Expect(k8sClient.Get(testCtx, key, &user)).To(Succeed())

				patch := client.MergeFrom(user.DeepCopy())
				patchFn(&user)

				err := k8sClient.Patch(testCtx, &user, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating MariaDBRef",
				func(umdb *v1alpha1.User) {
					umdb.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(umdb *v1alpha1.User) {
					umdb.Spec.PasswordSecretKeyRef.Name = "another-secret"
				},
				false,
			),
			Entry(
				"Updating MaxUserConnections",
				func(umdb *v1alpha1.User) {
					umdb.Spec.MaxUserConnections = 20
				},
				false,
			),
			Entry(
				"Duplicate authentication methods",
				func(umdb *v1alpha1.User) {
					umdb.Spec.PasswordHashSecretKeyRef = umdb.Spec.PasswordSecretKeyRef
				},
				true,
			),
			Entry(
				"Duplicate authentication methods",
				func(umdb *v1alpha1.User) {
					umdb.Spec.PasswordPlugin = PasswordPlugin
				},
				true,
			),
			Entry(
				"Updating PasswordPlugin",
				func(umdb *v1alpha1.User) {
					umdb.Spec.PasswordSecretKeyRef = nil
					umdb.Spec.PasswordPlugin = PasswordPlugin
				},
				false,
			),
			Entry(
				"Updating PasswordPlugin.PluginArgSecretKeyRef",
				func(umdb *v1alpha1.User) {
					umdb.Spec.PasswordSecretKeyRef = nil
					umdb.Spec.PasswordPlugin = PasswordPlugin
					umdb.Spec.PasswordPlugin.PluginArgSecretKeyRef.Name = "another-secret"
				},
				false,
			),
			Entry(
				"Updating CleanupPolicy",
				func(umdb *v1alpha1.User) {
					umdb.Spec.CleanupPolicy = ptr.To(v1alpha1.CleanupPolicySkip)
				},
				false,
			),
			Entry(
				"Updating TLSRequirements",
				func(umdb *v1alpha1.User) {
					umdb.Spec.Require = &v1alpha1.TLSRequirements{
						X509: ptr.To(true),
					}
				},
				false,
			),
		)
	})
})
