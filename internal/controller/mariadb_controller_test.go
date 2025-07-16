package controller

import (
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB spec", Label("basic"), func() {
	It("should default", func() {
		By("Creating MariaDB")
		key := types.NamespacedName{
			Name:      "test-mariadb-default",
			Namespace: testNamespace,
		}
		mdb := mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting to eventually default")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &mdb); err != nil {
				return false
			}
			return mdb.Spec.Image != "" && mdb.Spec.TLS != nil && mdb.Spec.TLS.Enabled
		}, testTimeout, testInterval).Should(BeTrue())
	})

	DescribeTable("should render default config",
		func(mariadb *mariadbv1alpha1.MariaDB, expectedConfig string) {
			config, err := defaultConfig(mariadb)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(BeEquivalentTo(expectedConfig))
		},
		Entry(
			"no timezone",
			&mariadbv1alpha1.MariaDB{},
			`[mariadb]
skip-name-resolve
temp-pool
ignore_db_dirs = 'lost+found'
`,
		),
		Entry(
			"timezone",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TimeZone: ptr.To("UTC"),
				},
			},
			`[mariadb]
skip-name-resolve
temp-pool
ignore_db_dirs = 'lost+found'
default_time_zone = UTC
`,
		),
	)
})

var _ = Describe("MariaDB", Label("basic"), func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should suspend", func() {
		By("Creating MariaDB")
		key := types.NamespacedName{
			Name:      "test-mariadb-suspend",
			Namespace: testNamespace,
		}
		mdb := mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Suspend MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, &mdb); err != nil {
				return false
			}
			mdb.Spec.Suspend = true

			return k8sClient.Update(testCtx, &mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually be suspended")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			condition := meta.FindStatusCondition(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReady)
			if condition == nil {
				return false
			}
			return condition.Status == metav1.ConditionFalse && condition.Reason == mariadbv1alpha1.ConditionReasonSuspended
		})
	})

	It("should reconcile", func() {
		var testMariaDB mariadbv1alpha1.MariaDB
		By("Getting MariaDB")
		Expect(k8sClient.Get(testCtx, testMdbkey, &testMariaDB)).To(Succeed())

		By("Expecting to create a ServiceAccount eventually")
		Eventually(func(g Gomega) bool {
			var svcAcc corev1.ServiceAccount
			key := testMariaDB.Spec.PodTemplate.ServiceAccountKey(testMariaDB.ObjectMeta)
			g.Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())

			g.Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a default ConfigMap eventually")
		Eventually(func(g Gomega) bool {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      testMariaDB.DefaultConfigMapKeyRef().Name,
				Namespace: testMariaDB.Namespace,
			}
			if err := k8sClient.Get(testCtx, key, &cm); err != nil {
				return false
			}
			g.Expect(cm.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(cm.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(cm.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(cm.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(cm.Data).To(HaveKeyWithValue("0-default.cnf", "[mariadb]\nskip-name-resolve\ntemp-pool\nignore_db_dirs = 'lost+found'\n"))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a ConfigMap eventually")
		Eventually(func(g Gomega) bool {
			if testMariaDB.Spec.MyCnfConfigMapKeyRef == nil {
				return false
			}
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      testMariaDB.Spec.MyCnfConfigMapKeyRef.Name,
				Namespace: testMariaDB.Namespace,
			}
			if err := k8sClient.Get(testCtx, key, &cm); err != nil {
				return false
			}
			g.Expect(cm.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(cm.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(cm.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(cm.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a StatefulSet eventually")
		Eventually(func(g Gomega) bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(testCtx, testMdbkey, &sts); err != nil {
				return false
			}
			g.Expect(sts.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(sts.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(sts.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(sts.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Pod to have metadata")
		Eventually(func(g Gomega) bool {
			key := types.NamespacedName{
				Name:      stsobj.PodName(testMariaDB.ObjectMeta, 0),
				Namespace: testMariaDB.Namespace,
			}
			var pod corev1.Pod
			if err := k8sClient.Get(testCtx, key, &pod); err != nil {
				return false
			}
			g.Expect(pod.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(pod.ObjectMeta.Labels).To(HaveKeyWithValue("sidecar.istio.io/inject", "false"))
			g.Expect(pod.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(pod.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return true
		}).WithTimeout(testTimeout).WithPolling(testInterval).Should(BeTrue())

		By("Expecting to create a Service eventually")
		Eventually(func(g Gomega) bool {
			var svc corev1.Service
			if err := k8sClient.Get(testCtx, testMdbkey, &svc); err != nil {
				return false
			}
			g.Expect(svc.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(svc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(svc.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(svc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Connection to be ready eventually")
		Eventually(func(g Gomega) bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDB), &conn); err != nil {
				return false
			}
			g.Expect(conn.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(conn.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(conn.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(conn.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a exporter Deployment eventually")
		Eventually(func(g Gomega) bool {
			var deploy appsv1.Deployment
			if err := k8sClient.Get(testCtx, testMariaDB.MetricsKey(), &deploy); err != nil {
				return false
			}
			expectedImage := os.Getenv("RELATED_IMAGE_EXPORTER")
			g.Expect(expectedImage).ToNot(BeEmpty())
			By("Expecting Deployment to have exporter image")
			g.Expect(deploy.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Image": Equal(expectedImage),
				})))

			g.Expect(deploy.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(deploy.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(deploy.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(deploy.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return deploymentReady(&deploy)
		}).WithTimeout(testTimeout).WithPolling(testInterval).Should(BeTrue())

		By("Expecting to create a ServiceMonitor eventually")
		Eventually(func(g Gomega) bool {
			var svcMonitor monitoringv1.ServiceMonitor
			if err := k8sClient.Get(testCtx, testMariaDB.MetricsKey(), &svcMonitor); err != nil {
				return false
			}

			g.Expect(svcMonitor.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(svcMonitor.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(svcMonitor.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(svcMonitor.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))

			g.Expect(svcMonitor.Spec.Selector.MatchLabels).NotTo(BeEmpty())
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "exporter"))
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", testMariaDB.MetricsKey().Name))
			g.Expect(svcMonitor.Spec.Endpoints).To(HaveLen(1))
			return true
		}).WithTimeout(testTimeout).WithPolling(testInterval).Should(BeTrue())
	})

	It("should reconcile SQL", func() {
		var testMariaDB mariadbv1alpha1.MariaDB
		By("Getting MariaDB")
		Expect(k8sClient.Get(testCtx, testMdbkey, &testMariaDB)).To(Succeed())

		By("Expecting initial Database to be ready eventually")
		Eventually(func(g Gomega) bool {
			var database mariadbv1alpha1.Database
			if err := k8sClient.Get(testCtx, testMariaDB.MariadbDatabaseKey(), &database); err != nil {
				return false
			}
			g.Expect(database.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(database.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(database.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(database.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return database.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting initial User to be ready eventually")
		Eventually(func(g Gomega) bool {
			var user mariadbv1alpha1.User
			if err := k8sClient.Get(testCtx, testMariaDB.MariadbUserKey(), &user); err != nil {
				return false
			}
			g.Expect(user.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(user.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(user.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(user.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting initial Grant to be ready eventually")
		Eventually(func(g Gomega) bool {
			var grant mariadbv1alpha1.Grant
			if err := k8sClient.Get(testCtx, testMariaDB.MariadbGrantKey(), &grant); err != nil {
				return false
			}
			g.Expect(grant.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(grant.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(grant.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(grant.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting mariadb.sys User to be ready eventually")
		Eventually(func(g Gomega) bool {
			var user mariadbv1alpha1.User
			if err := k8sClient.Get(testCtx, testMariaDB.MariadbSysUserKey(), &user); err != nil {
				return false
			}
			g.Expect(user.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(user.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(user.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(user.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting mariadb.sys Grant to be ready eventually")
		Eventually(func(g Gomega) bool {
			var grant mariadbv1alpha1.Grant
			if err := k8sClient.Get(testCtx, testMariaDB.MariadbSysGrantKey(), &grant); err != nil {
				return false
			}
			g.Expect(grant.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(grant.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(grant.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(grant.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting metrics User to be ready eventually")
		Eventually(func(g Gomega) bool {
			var user mariadbv1alpha1.User
			if err := k8sClient.Get(testCtx, testMariaDB.MetricsKey(), &user); err != nil {
				return false
			}
			g.Expect(user.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(user.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(user.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(user.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting metrics Grant to be ready eventually")
		Eventually(func(g Gomega) bool {
			var grant mariadbv1alpha1.Grant
			if err := k8sClient.Get(testCtx, testMariaDB.MetricsKey(), &grant); err != nil {
				return false
			}
			g.Expect(grant.ObjectMeta.Labels).NotTo(BeNil())
			g.Expect(grant.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			g.Expect(grant.ObjectMeta.Annotations).NotTo(BeNil())
			g.Expect(grant.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should reconcile with generated passwords", func() {
		key := types.NamespacedName{
			Name:      "mariadb-generate",
			Namespace: testNamespace,
		}
		rootPasswordKey := types.NamespacedName{
			Name:      "mariadb-root-generate",
			Namespace: testNamespace,
		}

		mdb := mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: rootPasswordKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Generate: true,
				},
				Username: ptr.To("user"),
				PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Generate: true,
				},
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
			},
		}
		applyMariadbTestConfig(&mdb)

		By("Creating MariaDB")
		Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting MariaDB to be ready eventually")
		expectMariadbReady(testCtx, k8sClient, key)

		By("Expecting to create a root Secret eventually")
		expectSecretToExist(testCtx, k8sClient, rootPasswordKey, testPwdSecretKey)

		By("Expecting to create a password Secret eventually")
		expectSecretToExist(testCtx, k8sClient, testPwdKey, testPwdSecretKey)
	})

	It("should get an update when my.cnf is updated", func() {
		By("Creating MariaDB")
		key := types.NamespacedName{
			Name:      "test-mariadb-mycnf",
			Namespace: testNamespace,
		}
		mdb := mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
				UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
					Type: mariadbv1alpha1.OnDeleteUpdateType,
				},
				MyCnf: ptr.To(`[mariadb]
				bind-address=*
				default_storage_engine=InnoDB
				binlog_format=row
				innodb_autoinc_lock_mode=2
				innodb_buffer_pool_size=1024M
				max_allowed_packet=256M
				`),
				TLS: &mariadbv1alpha1.TLS{
					Enabled:  true,
					Required: ptr.To(true),
				},
			},
		}
		applyMariadbTestConfig(&mdb)

		By("Creating MariaDB")
		Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting MariaDB to be ready eventually")
		expectMariadbReady(testCtx, k8sClient, key)

		By("Updating innodb_buffer_pool_size in my.cnf")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &mdb)).To(Succeed())
			mdb.Spec.MyCnf = ptr.To(`[mariadb]
			bind-address=*
			default_storage_engine=InnoDB
			binlog_format=row
			innodb_autoinc_lock_mode=2
			innodb_buffer_pool_size=2048M
			max_allowed_packet=256M
			`)
			g.Expect(k8sClient.Update(testCtx, &mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to have a pending update eventually")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.HasPendingUpdate()
		})
	})

	It("should get an update when my.cnf ConfigMap is updated", func() {
		key := types.NamespacedName{
			Name:      "test-mariadb-mycnf-configmap",
			Namespace: testNamespace,
		}

		By("Creating ConfigMap")
		configMapKey := "my.cnf"
		configMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			Data: map[string]string{
				configMapKey: `[mariadb]
				bind-address=*
				default_storage_engine=InnoDB
				binlog_format=row
				innodb_autoinc_lock_mode=2
				innodb_buffer_pool_size=1024M
				max_allowed_packet=256M
				`,
			},
		}
		Expect(k8sClient.Create(testCtx, &configMap)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &configMap)).To(Succeed())
		})

		By("Creating MariaDB")
		mdb := mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
				UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
					Type: mariadbv1alpha1.OnDeleteUpdateType,
				},
				MyCnfConfigMapKeyRef: &mariadbv1alpha1.ConfigMapKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: configMap.Name,
					},
					Key: configMapKey,
				},
				TLS: &mariadbv1alpha1.TLS{
					Enabled:  true,
					Required: ptr.To(true),
				},
			},
		}
		applyMariadbTestConfig(&mdb)

		By("Creating MariaDB")
		Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting MariaDB to be ready eventually")
		expectMariadbReady(testCtx, k8sClient, key)

		By("Updating innodb_buffer_pool_size in my.cnf ConfigMap")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &configMap)).To(Succeed())
			configMap.Data[configMapKey] = `[mariadb]
			bind-address=*
			default_storage_engine=InnoDB
			binlog_format=row
			innodb_autoinc_lock_mode=2
			innodb_buffer_pool_size=2048M
			max_allowed_packet=256M
			`
			g.Expect(k8sClient.Update(testCtx, &configMap)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to have a pending update eventually")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.HasPendingUpdate()
		})
	})

	It("should update metrics password", func() {
		var mdb mariadbv1alpha1.MariaDB
		By("Getting MariaDB")
		Expect(k8sClient.Get(testCtx, testMdbkey, &mdb)).To(Succeed())

		var deploy appsv1.Deployment
		By("Expecting exporter Deployment to be ready eventually")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, mdb.MetricsKey(), &deploy); err != nil {
				return false
			}
			return deploymentReady(&deploy)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Getting config hash")
		configHash := deploy.Spec.Template.Annotations[metadata.ConfigAnnotation]

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(testCtx, testPwdKey, &secret)).To(Succeed())
			secret.Data[testPwdMetricsSecretKey] = []byte("MariaDB12!")
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting config hash to be updated")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, mdb.MetricsKey(), &deploy); err != nil {
				return false
			}
			g.Expect(deploy.Spec.Template.Annotations[metadata.ConfigAnnotation]).NotTo(Equal(configHash))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting exporter Deployment to be ready eventually")
		Eventually(func(g Gomega) bool {
			if err := k8sClient.Get(testCtx, mdb.MetricsKey(), &deploy); err != nil {
				return false
			}
			return deploymentReady(&deploy)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	DescribeTable("should bootstrap from logical backup",
		func(backup *mariadbv1alpha1.Backup, bootstrapFrom mariadbv1alpha1.BootstrapFrom,
			mariadbKey types.NamespacedName) {
			backupKey := client.ObjectKeyFromObject(backup)

			By("Creating Backup")
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Bootstrapping MariaDB from Backup")
			testMariadbBootstrap(mariadbKey, &bootstrapFrom)
		},
		Entry(
			"Backup",
			getBackupWithS3Storage(
				types.NamespacedName{
					Name:      "test-backup",
					Namespace: testNamespace,
				},
				"test-mariadb",
				"",
			),
			newBootstrapFromRestoreSource(mariadbv1alpha1.RestoreSource{
				BackupRef: &mariadbv1alpha1.LocalObjectReference{
					Name: "test-backup",
				},
				TargetRecoveryTime: &metav1.Time{Time: time.Now()},
			}),
			types.NamespacedName{
				Name:      "test-mariadb-from-backup",
				Namespace: testNamespace,
			},
		),
		Entry(
			"Backup S3",
			getBackupWithS3Storage(
				types.NamespacedName{
					Name:      "test-backup-from-s3",
					Namespace: testNamespace,
				},
				"test-mariadb",
				"",
			),
			newBootstrapFromRestoreSource(mariadbv1alpha1.RestoreSource{
				S3: getS3WithBucket("test-mariadb", ""),
				StagingStorage: &mariadbv1alpha1.BackupStagingStorage{
					PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.MustParse("500Mi"),
							},
						},
					},
				},
				TargetRecoveryTime: &metav1.Time{Time: time.Now()},
			}),
			types.NamespacedName{
				Name:      "test-mariadb-from-s3",
				Namespace: testNamespace,
			},
		),
	)

	DescribeTable("should bootstrap from physical backup",
		func(backup *mariadbv1alpha1.PhysicalBackup, bootstrapFrom mariadbv1alpha1.BootstrapFrom,
			mariadbKey types.NamespacedName) {
			backupKey := client.ObjectKeyFromObject(backup)

			By("Creating PhysicalBackup")
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			By("Expecting PhysicalBackup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Bootstrapping MariaDB from PhysicalBackup")
			testMariadbBootstrap(mariadbKey, &bootstrapFrom)
		},
		Entry(
			"PhysicalBackup",
			buildPhysicalBackupWithS3Storage(
				testMdbkey,
				"test-mariadb-physical",
				"",
			)(
				types.NamespacedName{
					Name:      "test-physicalbackup",
					Namespace: testNamespace,
				},
			),
			mariadbv1alpha1.BootstrapFrom{
				BackupRef: &mariadbv1alpha1.TypedLocalObjectReference{
					Name: "test-physicalbackup",
					Kind: mariadbv1alpha1.PhysicalBackupKind,
				},
				TargetRecoveryTime: &metav1.Time{Time: time.Now()},
			},
			types.NamespacedName{
				Name:      "test-mariadb-from-physicalbackup",
				Namespace: testNamespace,
			},
		),
		Entry(
			"PhysicalBackup VolumeSnapshot",
			buildPhysicalBackupWithVolumeSnapshotStorage(testMdbkey)(
				types.NamespacedName{
					Name:      "test-physicalbackup-volumesnapshot",
					Namespace: testNamespace,
				},
			),
			mariadbv1alpha1.BootstrapFrom{
				BackupRef: &mariadbv1alpha1.TypedLocalObjectReference{
					Name: "test-physicalbackup-volumesnapshot",
					Kind: mariadbv1alpha1.PhysicalBackupKind,
				},
				TargetRecoveryTime: &metav1.Time{Time: time.Now()},
			},
			types.NamespacedName{
				Name:      "test-mariadb-from-physicalbackup-volumesnapshot",
				Namespace: testNamespace,
			},
		),
	)
})

func testMariadbBootstrap(key types.NamespacedName, bootstrapFrom *mariadbv1alpha1.BootstrapFrom) {
	mdb := mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
				SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
			},
			BootstrapFrom: bootstrapFrom,
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("100Mi")),
			},
			TLS: &mariadbv1alpha1.TLS{
				Enabled:  true,
				Required: ptr.To(true),
			},
		},
	}
	applyMariadbTestConfig(&mdb)

	By("Creating MariaDB")
	Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
	DeferCleanup(func() {
		deleteMariadb(key, false)
	})

	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, &mdb); err != nil {
			return false
		}
		return mdb.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting MariaDB to eventually have restored backup")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, &mdb); err != nil {
			return false
		}
		return mdb.HasRestoredBackup()
	}, testTimeout, testInterval).Should(BeTrue())
}

func newBootstrapFromRestoreSource(source mariadbv1alpha1.RestoreSource) mariadbv1alpha1.BootstrapFrom {
	var typedBackupRef *mariadbv1alpha1.TypedLocalObjectReference
	if source.BackupRef != nil {
		typedBackupRef = &mariadbv1alpha1.TypedLocalObjectReference{
			Name: source.BackupRef.Name,
			Kind: mariadbv1alpha1.BackupKind,
		}
	}
	return mariadbv1alpha1.BootstrapFrom{
		BackupRef:          typedBackupRef,
		S3:                 source.S3,
		Volume:             source.Volume,
		TargetRecoveryTime: source.TargetRecoveryTime,
		StagingStorage:     source.StagingStorage,
	}
}
