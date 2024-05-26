package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MaxScale", func() {
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
})
