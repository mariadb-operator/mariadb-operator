package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MaxScale", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should default", func() {
		By("Creating MaxScale")
		testDefaultMxsKey := types.NamespacedName{
			Name:      "test-maxscale-default",
			Namespace: testNamespace,
		}
		testDefaultMxs := mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testDefaultMxsKey.Name,
				Namespace: testDefaultMxsKey.Namespace,
			},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name:      testMdbkey.Name,
						Namespace: testMdbkey.Namespace,
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &testDefaultMxs)).To(Succeed())
		DeferCleanup(func() {
			deleteMaxScale(testDefaultMxsKey, false)
		})

		By("Expecting to eventually default")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, testDefaultMxsKey, &testDefaultMxs); err != nil {
				return false
			}
			return testDefaultMxs.Spec.Image != "" && len(testDefaultMxs.Spec.Servers) > 0 &&
				len(testDefaultMxs.Spec.Services) > 0 && testDefaultMxs.Spec.Monitor.Module != ""
		}, testTimeout, testInterval).Should(BeTrue())
	})

	// TODO: further tests using "mxs-test" instance will need to run serially with this one
	It("should update metrics password", func() {
		key := types.NamespacedName{
			Name:      "mxs-test",
			Namespace: testNamespace,
		}
		secretKey := "password"

		By("Creating Secret")
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			StringData: map[string]string{
				secretKey: "MaxScale11!",
			},
		}
		Expect(k8sClient.Create(testCtx, &secret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &secret)).To(Succeed())
		})

		By("Creating MaxScale")
		mxs := mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name:      testMdbkey.Name,
						Namespace: testMdbkey.Namespace,
					},
				},
				Metrics: &mariadbv1alpha1.MaxScaleMetrics{
					Enabled: true,
				},
				Auth: mariadbv1alpha1.MaxScaleAuth{
					MetricsUsername: "metrics",
					MetricsPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: secretKey,
						},
						Generate: false,
					},
				},
				KubernetesService: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.51",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &mxs)).To(Succeed())
		DeferCleanup(func() {
			deleteMaxScale(key, false)
		})

		By("Expecting MaxScale to be ready eventually")
		Eventually(func() bool {
			var mxs mariadbv1alpha1.MaxScale
			if err := k8sClient.Get(testCtx, key, &mxs); err != nil {
				return false
			}
			return mxs.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var deploy appsv1.Deployment
		By("Expecting exporter Deployment to be ready eventually")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &deploy); err != nil {
				return false
			}
			return deploymentReady(&deploy)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Getting config hash")
		configHash := deploy.Spec.Template.Annotations[metadata.ConfigAnnotation]

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &secret)).To(Succeed())
			secret.Data[secretKey] = []byte("MaxScale12!")
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting config hash to be updated")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &deploy); err != nil {
				return false
			}
			g.Expect(deploy.Spec.Template.Annotations[metadata.ConfigAnnotation]).NotTo(Equal(configHash))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting exporter Deployment to be ready eventually")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &deploy); err != nil {
				return false
			}
			return deploymentReady(&deploy)
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
