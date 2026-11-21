package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("User types", func() {
	Context("When checking client certificate collisions", func() {
		newUser := func(name, namespace string) *User {
			return &User{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		}
		mdb := &MariaDB{ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "ns-a"}}

		It("should not collide with a differently named User", func() {
			Expect(newUser("app", "ns-a").ClientCertCollidesWith(mdb)).To(BeFalse())
			Expect(newUser("app", "ns-b").ClientCertCollidesWith(mdb)).To(BeFalse())
		})

		It("should collide with a User named like the MariaDB, in any namespace", func() {
			Expect(newUser("mariadb", "ns-a").ClientCertCollidesWith(mdb)).To(BeTrue())
			Expect(newUser("mariadb", "ns-b").ClientCertCollidesWith(mdb)).To(BeTrue())
		})

		It("should collide when the Secret is the MariaDB's custom client cert Secret", func() {
			byoMdb := mdb.DeepCopy()
			byoMdb.Spec.TLS = &TLS{
				Enabled:             true,
				ClientCertSecretRef: &LocalObjectReference{Name: "app-client-cert"},
			}
			Expect(newUser("app", "ns-a").ClientCertCollidesWith(byoMdb)).To(BeTrue())
			Expect(newUser("app", "ns-b").ClientCertCollidesWith(byoMdb)).To(BeFalse())
		})
	})
})
