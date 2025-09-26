package controller

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
				TLS: &mariadbv1alpha1.TLS{
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
				TLS: &mariadbv1alpha1.TLS{
					Enabled:  true,
					Required: ptr.To(false),
					Mutual:   ptr.To(false),
					ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
						Name: "mdb-emulate-external-test-ca",
					},
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
