package controller

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A single MaxScale runs without cooperative monitoring locks, so its monitor reports no primary flag at all. The
// operator only drove switchovers through a monitor reporting "primary: true", so every switchover it requested
// through MaxScale, including the one a rolling update of the MariaDB Pods needs, waited forever.
var _ = Describe("MariaDB switchover through a single MaxScale", Ordered, func() {
	var (
		mdbKey = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		mxsKey = types.NamespacedName{Name: "maxscale-repl", Namespace: testNamespace}
		mdb    *mariadbv1alpha1.MariaDB
		mxs    *mariadbv1alpha1.MaxScale
	)

	BeforeAll(func() {
		mdb = buildTestMariaDBWithRepl(mdbKey)
		mdb.Spec.Replicas = 2
		applyMariadbTestConfig(mdb)
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(mdbKey, true)
		})
		Eventually(func() bool {
			return k8sClient.Get(testCtx, mdbKey, mdb) == nil && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		mxs = &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{Name: mxsKey.Name, Namespace: mxsKey.Namespace},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				Replicas: 1,
				KubernetesService: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.214",
						},
					},
				},
				Connection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: ptr.To("mxs-switchover-conn"),
					HealthCheck: &mariadbv1alpha1.HealthCheck{
						Interval: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
					},
				},
				Auth: mariadbv1alpha1.MaxScaleAuth{
					Generate: ptr.To(true),
					AdminPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{Name: testPwdKey.Name},
							Key:                  testPwdSecretKey,
						},
						Generate: false,
					},
				},
				TLS: &mariadbv1alpha1.MaxScaleTLS{
					Enabled:               true,
					VerifyPeerCertificate: ptr.To(true),
					VerifyPeerHost:        ptr.To(false),
					ReplicationSSLEnabled: ptr.To(true),
				},
			},
		}
		testMaxscale(mdb, mxs)
	})

	It("switches the primary requested through spec.primaryServer", func() {
		Expect(k8sClient.Get(testCtx, mxsKey, mxs)).To(Succeed())
		Expect(mxs.Spec.Monitor.CooperativeMonitoring).To(BeNil())
		current := ptr.Deref(mxs.Status.PrimaryServer, "")
		Expect(current).ToNot(BeEmpty())
		target := "mariadb-repl-0"
		if current == target {
			target = "mariadb-repl-1"
		}

		By("Requesting a switchover the way the MariaDB update does")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, mxsKey, mxs)).To(Succeed())
			patch := client.MergeFrom(mxs.DeepCopy())
			mxs.Spec.PrimaryServer = ptr.To(target)
			g.Expect(k8sClient.Patch(testCtx, mxs, patch)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MaxScale to switch the primary and clear the request")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, mxsKey, mxs)).To(Succeed())
			return ptr.Deref(mxs.Status.PrimaryServer, "") == target && mxs.Spec.PrimaryServer == nil
		}, 3*time.Minute, testInterval).Should(BeTrue())

		By("Expecting the MariaDB primary to follow")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
				return false
			}
			return ptr.Deref(mdb.Status.CurrentPrimary, "") == target && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())
	})
})
