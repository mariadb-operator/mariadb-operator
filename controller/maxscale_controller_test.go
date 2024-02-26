package controller

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
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
	})

	Context("When creating a MariaDB replication with MaxScale", Serial, func() {
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
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Replicas: 3,
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
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
						Enabled:  true,
						Replicas: ptr.To(int32(3)),
						KubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.64",
							},
						},
						Connection: &mariadbv1alpha1.ConnectionTemplate{
							SecretName: ptr.To("mxs-repl-conn"),
							HealthCheck: &mariadbv1alpha1.HealthCheck{
								Interval: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
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
			expecFailoverSuccess(&testMdbMxs)
		})
	})

	Context("When creating a MariaDB Galera with MaxScale", Serial, func() {
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
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					Replicas: 3,
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
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
						Enabled:  true,
						Replicas: ptr.To(int32(3)),
						KubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.84",
							},
						},
						Connection: &mariadbv1alpha1.ConnectionTemplate{
							SecretName: ptr.To("mxs-galera-conn"),
							HealthCheck: &mariadbv1alpha1.HealthCheck{
								Interval: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
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
			}, testHighTimeout, testInterval).Should(BeTrue())

			expectMaxScaleReady(testMdbMxs.MaxScaleKey())
			expecFailoverSuccess(&testMdbMxs)
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

	By("Expecting Connection to be ready eventually")
	Eventually(func() bool {
		var conn mariadbv1alpha1.Connection
		if err := k8sClient.Get(testCtx, mxs.ConnectionKey(), &conn); err != nil {
			return false
		}
		return conn.IsReady()
	}, testTimeout, testInterval).Should(BeTrue())
}

func expecFailoverSuccess(mdb *mariadbv1alpha1.MariaDB) {
	var (
		mxs             mariadbv1alpha1.MaxScale
		previousPrimary string
	)
	By("Expecting primary to be set eventually")
	Eventually(func(g Gomega) bool {
		Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(mdb), mdb)).Should(Succeed())
		Expect(mdb.Status.CurrentPrimary).ToNot(BeNil())
		Expect(mdb.Status.CurrentPrimaryPodIndex).ToNot(BeNil())
		previousPrimary = *mdb.Status.CurrentPrimary

		Expect(k8sClient.Get(testCtx, mdb.MaxScaleKey(), &mxs)).Should(Succeed())
		Expect(mxs.Status.PrimaryServer).NotTo(BeNil())
		return true
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting primary to have changed eventually")
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(mdb), mdb)).Should(Succeed())
		g.Expect(k8sClient.Get(testCtx, mdb.MaxScaleKey(), &mxs)).Should(Succeed())

		g.Expect(mdb.Status.CurrentPrimary).NotTo(BeNil())
		g.Expect(mdb.Status.CurrentPrimaryPodIndex).NotTo(BeNil())
		g.Expect(mxs.Status.PrimaryServer).NotTo(BeNil())

		g.Expect(deletePod(previousPrimary)).To(Succeed())

		podIndex, err := podIndexForServer(*mxs.Status.PrimaryServer, &mxs, mdb)
		primary := stsobj.PodName(mdb.ObjectMeta, *podIndex)
		g.Expect(podIndex).NotTo(BeNil())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(primary).NotTo(Equal(previousPrimary))
		return true
	}, testHighTimeout, testInterval).Should(BeTrue())
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

func deletePod(podName string) error {
	key := types.NamespacedName{
		Name:      podName,
		Namespace: testNamespace,
	}
	var pod corev1.Pod
	if err := k8sClient.Get(testCtx, key, &pod); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := k8sClient.Delete(testCtx, &pod); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}
