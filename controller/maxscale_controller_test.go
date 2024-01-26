package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
					MaxScaleBaseSpec: mariadbv1alpha1.MaxScaleBaseSpec{
						Services: []mariadbv1alpha1.MaxScaleService{
							{
								Name:   "rw-router",
								Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
								Listener: mariadbv1alpha1.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: mariadbv1alpha1.MaxScaleMonitor{
							Module: mariadbv1alpha1.MonitorModuleMariadb,
						},
					},
					Servers: []mariadbv1alpha1.MaxScaleServer{
						{
							Name:    "mariadb-0",
							Address: "mariadb-0.mariadb-internal.default.svc.cluster.local",
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
				return testDefaultMaxScale.Spec.Image != ""
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting MaxScale")
			Expect(k8sClient.Delete(testCtx, &testDefaultMaxScale)).To(Succeed())
		})

		It("Should reconcile", func() {
			By("Creating MaxScale")
			testMaxScaleKey := types.NamespacedName{
				Name:      "test-maxscale",
				Namespace: testNamespace,
			}
			testMaxScale := mariadbv1alpha1.MaxScale{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testMaxScaleKey.Name,
					Namespace: testMaxScaleKey.Namespace,
				},
				Spec: mariadbv1alpha1.MaxScaleSpec{
					MaxScaleBaseSpec: mariadbv1alpha1.MaxScaleBaseSpec{
						Services: []mariadbv1alpha1.MaxScaleService{
							{
								Name:   "rw-router",
								Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
								Listener: mariadbv1alpha1.MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: mariadbv1alpha1.MaxScaleMonitor{
							Module: mariadbv1alpha1.MonitorModuleMariadb,
						},
					},
					Servers: []mariadbv1alpha1.MaxScaleServer{
						{
							Name:    "mariadb-0",
							Address: "mariadb-0.mariadb-internal.default.svc.cluster.local",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &testMaxScale)).To(Succeed())

			By("Expecting to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, testMaxScaleKey, &testMaxScale); err != nil {
					return false
				}
				return testMaxScale.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Secret eventually")
			Eventually(func() bool {
				var secret corev1.Secret
				key := types.NamespacedName{
					Name:      testMaxScale.ConfigSecretKeyRef().Name,
					Namespace: testMaxScale.Namespace,
				}
				return k8sClient.Get(testCtx, key, &secret) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a StatefulSet eventually")
			Eventually(func() bool {
				var sts appsv1.StatefulSet
				return k8sClient.Get(testCtx, testMaxScaleKey, &sts) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service eventually")
			Eventually(func() bool {
				var svc corev1.Service
				return k8sClient.Get(testCtx, testMaxScaleKey, &svc) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting MaxScale")
			Expect(k8sClient.Delete(testCtx, &testMaxScale)).To(Succeed())
		})
	})
})
