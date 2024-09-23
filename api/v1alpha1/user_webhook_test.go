package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("User webhook", func() {
	Context("When creating a User", func() {
		key := types.NamespacedName{
			Name:      "user-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(user *User, wantErr bool) {
				err := k8sClient.Create(testCtx, user)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Valid cleanupPolicy",
				&User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: UserSpec{
						SQLTemplate: SQLTemplate{
							CleanupPolicy: ptr.To(CleanupPolicyDelete),
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						PasswordSecretKeyRef: &SecretKeySelector{
							LocalObjectReference: LocalObjectReference{
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
				&User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: UserSpec{
						SQLTemplate: SQLTemplate{
							CleanupPolicy: ptr.To(CleanupPolicy("")),
						},
						MariaDBRef: MariaDBRef{
							ObjectReference: ObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						PasswordSecretKeyRef: &SecretKeySelector{
							LocalObjectReference: LocalObjectReference{
								Name: "user-mariadb-webhook-root",
							},
							Key: "password",
						},
						MaxUserConnections: 10,
					},
				},
				true,
			),
		)
	})

	Context("When updating a User", Ordered, func() {
		key := types.NamespacedName{
			Name:      "user-update-webhook",
			Namespace: testNamespace,
		}
		PasswordPlugin := PasswordPlugin{
			PluginNameSecretKeyRef: &SecretKeySelector{
				LocalObjectReference: LocalObjectReference{
					Name: "user-mariadb-webhook-root",
				},
				Key: "pluginName",
			},
			PluginArgSecretKeyRef: &SecretKeySelector{
				LocalObjectReference: LocalObjectReference{
					Name: "user-mariadb-webhook-root",
				},
				Key: "pluginArg",
			},
		}
		BeforeAll(func() {
			user := User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: UserSpec{
					MariaDBRef: MariaDBRef{
						ObjectReference: ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					PasswordSecretKeyRef: &SecretKeySelector{
						LocalObjectReference: LocalObjectReference{
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
			func(patchFn func(u *User), wantErr bool) {
				var user User
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
				func(umdb *User) {
					umdb.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(umdb *User) {
					umdb.Spec.PasswordSecretKeyRef.Name = "another-secret"
				},
				false,
			),
			Entry(
				"Updating MaxUserConnections",
				func(umdb *User) {
					umdb.Spec.MaxUserConnections = 20
				},
				true,
			),
			Entry(
				"Duplicate authentication methods",
				func(umdb *User) {
					umdb.Spec.PasswordHashSecretKeyRef = umdb.Spec.PasswordSecretKeyRef
				},
				true,
			),
			Entry(
				"Duplicate authentication methods",
				func(umdb *User) {
					umdb.Spec.PasswordPlugin = PasswordPlugin
				},
				true,
			),
			Entry(
				"Updating PasswordPlugin",
				func(umdb *User) {
					umdb.Spec.PasswordSecretKeyRef = nil
					umdb.Spec.PasswordPlugin = PasswordPlugin
				},
				false,
			),
			Entry(
				"Updating PasswordPlugin.PluginArgSecretKeyRef",
				func(umdb *User) {
					umdb.Spec.PasswordSecretKeyRef = nil
					umdb.Spec.PasswordPlugin = PasswordPlugin
					umdb.Spec.PasswordPlugin.PluginArgSecretKeyRef.Name = "another-secret"
				},
				false,
			),
			Entry(
				"Updating to valid CleanupPolicy",
				func(umdb *User) {
					umdb.Spec.CleanupPolicy = ptr.To(CleanupPolicySkip)
				},
				false,
			),
			Entry(
				"Updating to invalid CleanupPolicy",
				func(umdb *User) {
					umdb.Spec.CleanupPolicy = ptr.To(CleanupPolicy(""))
				},
				true,
			),
		)
	})
})
