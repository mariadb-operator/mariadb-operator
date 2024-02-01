package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("MaxScale controller", func() {
	Context("When creating a MaxScale", func() {
		It("Should default", func() {
			By("Creating MaxScale")
			testDefaultKey := types.NamespacedName{
				Name:      "test-maxscale-default",
				Namespace: testNamespace,
			}
			testDefaultMaxScale := mariadbv1alpha1.MaxScale{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testDefaultKey.Name,
					Namespace: testDefaultKey.Namespace,
				},
				Spec: mariadbv1alpha1.MaxScaleSpec{
					MariaDBRef: &mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name:      testMariaDbKey.Name,
							Namespace: testMariaDbKey.Namespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &testDefaultMaxScale)).To(Succeed())

			By("Expecting to eventually default")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, testDefaultKey, &testDefaultMaxScale); err != nil {
					return false
				}
				return testDefaultMaxScale.Spec.Image != "" && len(testDefaultMaxScale.Spec.Servers) > 0 &&
					len(testDefaultMaxScale.Spec.Services) > 0 && testDefaultMaxScale.Spec.Monitor.Module != ""
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting MaxScale")
			Expect(k8sClient.Delete(testCtx, &testDefaultMaxScale)).To(Succeed())
		})

		It("Should reconcile", func() {
			By("Creating MaxScale")
			testMaxScaleKey := types.NamespacedName{
				Name:      "maxscale",
				Namespace: testNamespace,
			}
			testMaxScale := mariadbv1alpha1.MaxScale{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testMaxScaleKey.Name,
					Namespace: testMaxScaleKey.Namespace,
				},
				Spec: mariadbv1alpha1.MaxScaleSpec{
					MariaDBRef: &mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name:      testMariaDbKey.Name,
							Namespace: testMariaDbKey.Namespace,
						},
					},
					MaxScaleBaseSpec: mariadbv1alpha1.MaxScaleBaseSpec{
						KubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.214",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &testMaxScale)).To(Succeed())

			expectMaxScaleReady(testMaxScaleKey)

			By("Deleting MaxScale")
			Expect(k8sClient.Delete(testCtx, &testMaxScale)).To(Succeed())
		})
	})
})

func expectMaxScaleReady(key types.NamespacedName) {
	var mxs mariadbv1alpha1.MaxScale

	By("Expecting MaxScale to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, &mxs); err != nil {
			return false
		}
		return mxs.IsReady()
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting servers to be ready eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, key, &mxs); err != nil {
			return false
		}
		for _, srv := range mxs.Status.Servers {
			g.Expect(srv.IsReady()).To(BeTrue())
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting monitor to be running eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, key, &mxs); err != nil {
			return false
		}
		g.Expect(ptr.Deref(
			mxs.Status.Monitor,
			mariadbv1alpha1.MaxScaleResourceStatus{},
		).State).To(Equal("Running"))
		return true
	}).Should(BeTrue())

	By("Expecting services to be started eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, key, &mxs); err != nil {
			return false
		}
		for _, svc := range mxs.Status.Services {
			g.Expect(svc.State).To(Equal("Started"))
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting listeners to be running")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, key, &mxs); err != nil {
			return false
		}
		for _, listener := range mxs.Status.Listeners {
			g.Expect(listener.State).To(Equal("Running"))
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting to create a StatefulSet")
	var sts appsv1.StatefulSet
	Expect(k8sClient.Get(testCtx, key, &sts)).To(Succeed())

	By("Expecting to create a Service")
	var svc corev1.Service
	Expect(k8sClient.Get(testCtx, key, &svc)).To(Succeed())
}
