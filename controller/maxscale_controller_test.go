package controller

import (
	"fmt"
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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

	Context("with MariaDB replication", Ordered, func() {
		var (
			key = types.NamespacedName{
				Name:      "mxs-repl",
				Namespace: testNamespace,
			}
			mdbMxs *mariadbv1alpha1.MariaDB
		)

		BeforeAll(func() {
			mdbMxs = &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
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
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.54",
							},
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.55",
							},
						},
					},
					MaxScale: &mariadbv1alpha1.MariaDBMaxScaleSpec{
						Enabled:  true,
						Replicas: ptr.To(int32(3)),
						KubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Metadata: &mariadbv1alpha1.Metadata{
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.64",
								},
							},
						},
						GuiKubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Metadata: &mariadbv1alpha1.Metadata{
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.230",
								},
							},
						},
						Connection: &mariadbv1alpha1.ConnectionTemplate{
							SecretName: ptr.To("mxs-repl-conn"),
							HealthCheck: &mariadbv1alpha1.HealthCheck{
								Interval: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
							},
						},
						Auth: &mariadbv1alpha1.MaxScaleAuth{
							Generate: ptr.To(true),
							AdminPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: testPwdKey.Name,
									},
									Key: testPwdSecretKey,
								},
								Generate: false,
							},
						},
						Metrics: &mariadbv1alpha1.MaxScaleMetrics{
							Enabled: true,
						},
					},
				},
			}
			applyMariadbTestConfig(mdbMxs)

			By("Creating MariaDB replication with MaxScale")
			Expect(k8sClient.Create(testCtx, mdbMxs)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(mdbMxs)
				deleteMaxScale(mdbMxs.MaxScaleKey(), true)
			})
		})

		It("should reconcile", func() {
			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdbMxs); err != nil {
					return false
				}
				return mdbMxs.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MaxScale to reconcile")
			testMaxscale(mdbMxs.MaxScaleKey())
		})

		It("should fail over", FlakeAttempts(3), func() {
			By("Failing over MaxScale")
			testMaxscaleFailover(mdbMxs)
		})
	})

	Context("with MariaDB Galera", Ordered, func() {
		var (
			key = types.NamespacedName{
				Name:      "mxs-galera",
				Namespace: testNamespace,
			}
			mdbMxs *mariadbv1alpha1.MariaDB
		)

		BeforeAll(func() {
			mdbMxs = &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
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
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.74",
							},
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.75",
							},
						},
					},
					MaxScale: &mariadbv1alpha1.MariaDBMaxScaleSpec{
						Enabled:  true,
						Replicas: ptr.To(int32(3)),
						KubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Metadata: &mariadbv1alpha1.Metadata{
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.84",
								},
							},
						},
						GuiKubernetesService: &mariadbv1alpha1.ServiceTemplate{
							Type: corev1.ServiceTypeLoadBalancer,
							Metadata: &mariadbv1alpha1.Metadata{
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.231",
								},
							},
						},
						Connection: &mariadbv1alpha1.ConnectionTemplate{
							SecretName: ptr.To("mxs-galera-conn"),
							HealthCheck: &mariadbv1alpha1.HealthCheck{
								Interval: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
							},
						},
						Auth: &mariadbv1alpha1.MaxScaleAuth{
							Generate: ptr.To(true),
							AdminPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
								SecretKeySelector: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: testPwdKey.Name,
									},
									Key: testPwdSecretKey,
								},
								Generate: false,
							},
						},
						Metrics: &mariadbv1alpha1.MaxScaleMetrics{
							Enabled: true,
						},
					},
				},
			}
			applyMariadbTestConfig(mdbMxs)

			By("Creating MariaDB Galera with MaxScale")
			Expect(k8sClient.Create(testCtx, mdbMxs)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(mdbMxs)
				deleteMaxScale(mdbMxs.MaxScaleKey(), true)
			})
		})

		It("should reconcile", func() {
			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdbMxs); err != nil {
					return false
				}
				return mdbMxs.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MaxScale to reconcile")
			testMaxscale(mdbMxs.MaxScaleKey())
		})

		It("should fail over", func() {
			By("Failing over MaxScale")
			testMaxscaleFailover(mdbMxs)
		})
	})
})

func testMaxscale(key types.NamespacedName) {
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

	By("Expecting to create a ServiceAccount")
	var svcAcc corev1.ServiceAccount
	Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())

	By("Expecting to create a StatefulSet")
	var sts appsv1.StatefulSet
	Expect(k8sClient.Get(testCtx, key, &sts)).To(Succeed())

	By("Expecting to create a Service")
	var svc corev1.Service
	Expect(k8sClient.Get(testCtx, key, &svc)).To(Succeed())

	By("Expecting to create a GUI Service")
	var guiSvc corev1.Service
	Expect(k8sClient.Get(testCtx, mxs.GuiServiceKey(), &guiSvc)).To(Succeed())

	By("Expecting Connection to be ready eventually")
	Eventually(func() bool {
		var conn mariadbv1alpha1.Connection
		if err := k8sClient.Get(testCtx, mxs.ConnectionKey(), &conn); err != nil {
			return false
		}
		return conn.IsReady()
	}, testTimeout, testInterval).Should(BeTrue())

	type secretRef struct {
		name        string
		keySelector corev1.SecretKeySelector
	}
	secretKeyRefs := []secretRef{
		{
			name:        "admin",
			keySelector: mxs.Spec.Auth.AdminPasswordSecretKeyRef.SecretKeySelector,
		},
		{
			name:        "client",
			keySelector: mxs.Spec.Auth.ClientPasswordSecretKeyRef.SecretKeySelector,
		},
		{
			name:        "server",
			keySelector: mxs.Spec.Auth.ServerPasswordSecretKeyRef.SecretKeySelector,
		},
		{
			name:        "monitor",
			keySelector: mxs.Spec.Auth.MonitorPasswordSecretKeyRef.SecretKeySelector,
		},
	}
	if mxs.IsHAEnabled() {
		secretKeyRefs = append(secretKeyRefs, secretRef{
			name:        "sync",
			keySelector: mxs.Spec.Auth.SyncPasswordSecretKeyRef.SecretKeySelector,
		})
	}
	if mxs.AreMetricsEnabled() {
		secretKeyRefs = append(secretKeyRefs, secretRef{
			name:        "metrics",
			keySelector: mxs.Spec.Auth.MetricsPasswordSecretKeyRef.SecretKeySelector,
		})
	}

	for _, secretKeyRef := range secretKeyRefs {
		By(fmt.Sprintf("Expecting to create a '%s' Secret eventually", secretKeyRef.name))
		key := types.NamespacedName{
			Name:      secretKeyRef.keySelector.Name,
			Namespace: mxs.Namespace,
		}
		expectSecretToExist(testCtx, k8sClient, key, secretKeyRef.keySelector.Key)
	}

	if mxs.AreMetricsEnabled() {
		By("Expecting to create a exporter Deployment eventually")
		Eventually(func(g Gomega) bool {
			var deploy appsv1.Deployment
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &deploy); err != nil {
				return false
			}
			expectedImage := os.Getenv("RELATED_IMAGE_EXPORTER_MAXSCALE")
			g.Expect(expectedImage).ToNot(BeEmpty())

			By("Expecting Deployment to have exporter image")
			g.Expect(deploy.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Image": Equal(expectedImage),
				})))

			By("Expecting Deployment to be ready")
			return deploymentReady(&deploy)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a ServiceMonitor eventually")
		Eventually(func(g Gomega) bool {
			var svcMonitor monitoringv1.ServiceMonitor
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &svcMonitor); err != nil {
				return false
			}
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).NotTo(BeEmpty())
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "exporter"))
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", mxs.MetricsKey().Name))
			g.Expect(svcMonitor.Spec.Endpoints).To(HaveLen(int(mxs.Spec.Replicas)))
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	}
}

func testMaxscaleFailover(mdb *mariadbv1alpha1.MariaDB) {
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

func deleteMaxScale(key types.NamespacedName, assertPVCDeletion bool) {
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

	if !assertPVCDeletion {
		return
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
		err := k8sClient.List(testCtx, pvcList, listOpts)
		if err != nil && !apierrors.IsNotFound(err) {
			g.Expect(err).ToNot(HaveOccurred())
		}
		return len(pvcList.Items) == 0
	}, testHighTimeout, testInterval).Should(BeTrue())
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
