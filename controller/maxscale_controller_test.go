package controller

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MaxScale controller", func() {
	Context("When creating a MaxScale", func() {
		It("Should default", func() {
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
				deleteMaxScale(testDefaultMxsKey)
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
							Name:      testMdbkey.Name,
							Namespace: testMdbkey.Namespace,
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
			DeferCleanup(func() {
				deleteMaxScale(testMaxScaleKey)
			})

			expectMaxScaleReady(testMaxScaleKey)
		})
	})

	Context("When creating a MariaDB replication with MaxScale", func() {
		It("Should reconcile", func() {
			testMdbMxsKey := types.NamespacedName{
				Name:      "mxs-repl",
				Namespace: testNamespace,
			}
			testMdbMxs := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testMdbMxsKey.Name,
					Namespace: testMdbMxsKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					VolumeClaimTemplate: mariadbv1alpha1.VolumeClaimTemplate{
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("100Mi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
						},
					},
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Replicas: 3,
					Service: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.54",
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.55",
						},
					},
					MaxScale: &mariadbv1alpha1.MariaDBMaxScaleSpec{
						Enabled: true,
						MaxScaleBaseSpec: mariadbv1alpha1.MaxScaleBaseSpec{
							Replicas: 3,
							KubernetesService: &mariadbv1alpha1.ServiceTemplate{
								Type: corev1.ServiceTypeLoadBalancer,
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.64",
								},
							},
						},
					},
				},
			}
			By("Creating MariaDB replication with MaxScale")
			Expect(k8sClient.Create(testCtx, &testMdbMxs)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&testMdbMxs)
				deleteMaxScale(testMdbMxs.MaxScaleKey())
			})

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, testMdbMxsKey, &testMdbMxs); err != nil {
					return false
				}
				return testMdbMxs.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			expectMaxScaleReady(testMdbMxs.MaxScaleKey())
		})
	})

	Context("When creating a MariaDB Galera with MaxScale", func() {
		It("Should reconcile", func() {
			testMdbMxsKey := types.NamespacedName{
				Name:      "mxs-galera",
				Namespace: testNamespace,
			}
			testMdbMxs := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testMdbMxsKey.Name,
					Namespace: testMdbMxsKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					VolumeClaimTemplate: mariadbv1alpha1.VolumeClaimTemplate{
						PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse("100Mi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
						},
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					Replicas: 3,
					Service: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.74",
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.75",
						},
					},
					MaxScale: &mariadbv1alpha1.MariaDBMaxScaleSpec{
						Enabled: true,
						MaxScaleBaseSpec: mariadbv1alpha1.MaxScaleBaseSpec{
							Replicas: 3,
							KubernetesService: &mariadbv1alpha1.ServiceTemplate{
								Type: corev1.ServiceTypeLoadBalancer,
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.84",
								},
							},
						},
					},
				},
			}
			By("Creating MariaDB Galera with MaxScale")
			Expect(k8sClient.Create(testCtx, &testMdbMxs)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&testMdbMxs)
				deleteMaxScale(testMdbMxs.MaxScaleKey())
			})

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, testMdbMxsKey, &testMdbMxs); err != nil {
					return false
				}
				return testMdbMxs.IsReady()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			expectMaxScaleReady(testMdbMxs.MaxScaleKey())
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
	}, testHighTimeout, testInterval).Should(BeTrue())

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
	}, testTimeout, testInterval).Should(BeTrue())

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

func deleteMaxScale(key types.NamespacedName) {
	mxs := mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	err := k8sClient.Delete(testCtx, &mxs)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	Eventually(func(g Gomega) bool {
		listOpts := &client.ListOptions{
			LabelSelector: klabels.SelectorFromSet(
				labels.NewLabelsBuilder().
					WithMaxScaleSelectorLabels(&mxs).
					Build(),
			),
			Namespace: mxs.GetNamespace(),
		}
		pvcList := &corev1.PersistentVolumeClaimList{}
		g.Expect(k8sClient.List(testCtx, pvcList, listOpts)).To(Succeed())

		for _, pvc := range pvcList.Items {
			g.Expect(k8sClient.Delete(testCtx, &pvc)).To(Succeed())
		}
		return true
	}, 30*time.Second, 1*time.Second).Should(BeTrue())
}
