package controller

import (
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	jobpkg "github.com/mariadb-operator/mariadb-operator/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB controller", func() {
	Context("When creating a MariaDB", func() {
		It("Should default", func() {
			By("Creating MariaDB")
			testDefaultKey := types.NamespacedName{
				Name:      "test-mariadb-default",
				Namespace: testNamespace,
			}
			testDefaultMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testDefaultKey.Name,
					Namespace: testDefaultKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &testDefaultMariaDb)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&testDefaultMariaDb)
			})

			By("Expecting to eventually default")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, testDefaultKey, &testDefaultMariaDb); err != nil {
					return false
				}
				return testDefaultMariaDb.Spec.Image != ""
			}, testTimeout, testInterval).Should(BeTrue())
		})
		It("Should reconcile", func() {
			var testMariaDb mariadbv1alpha1.MariaDB
			By("Getting MariaDB")
			Expect(k8sClient.Get(testCtx, testMdbkey, &testMariaDb)).To(Succeed())

			By("Expecting to create a ServiceAccount eventually")
			Eventually(func(g Gomega) bool {
				var svcAcc corev1.ServiceAccount
				key := testMariaDb.Spec.PodTemplate.ServiceAccountKey(testMariaDb.ObjectMeta)
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
					Name:      testMariaDb.DefaultConfigMapKeyRef().Name,
					Namespace: testMariaDb.Namespace,
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

			By("Expecting to create a ConfigMap eventually")
			Eventually(func(g Gomega) bool {
				if testMariaDb.Spec.MyCnfConfigMapKeyRef == nil {
					return false
				}
				var cm corev1.ConfigMap
				key := types.NamespacedName{
					Name:      testMariaDb.Spec.MyCnfConfigMapKeyRef.Name,
					Namespace: testMariaDb.Namespace,
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
					Name:      stsobj.PodName(testMariaDb.ObjectMeta, 0),
					Namespace: testMariaDb.Namespace,
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
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDb), &conn); err != nil {
					return false
				}
				g.Expect(conn.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(conn.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
				g.Expect(conn.ObjectMeta.Annotations).NotTo(BeNil())
				g.Expect(conn.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting mariadb.sys User to be ready eventually")
			Eventually(func(g Gomega) bool {
				var user mariadbv1alpha1.User
				if err := k8sClient.Get(testCtx, testMariaDb.MariadbSysUserKey(), &user); err != nil {
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
				if err := k8sClient.Get(testCtx, testMariaDb.MariadbSysGrantKey(), &grant); err != nil {
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
				if err := k8sClient.Get(testCtx, testMariaDb.MetricsKey(), &user); err != nil {
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
				if err := k8sClient.Get(testCtx, testMariaDb.MetricsKey(), &grant); err != nil {
					return false
				}
				g.Expect(grant.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(grant.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
				g.Expect(grant.ObjectMeta.Annotations).NotTo(BeNil())
				g.Expect(grant.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
				return grant.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a exporter Deployment eventually")
			Eventually(func(g Gomega) bool {
				var deploy appsv1.Deployment
				if err := k8sClient.Get(testCtx, testMariaDb.MetricsKey(), &deploy); err != nil {
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
				if err := k8sClient.Get(testCtx, testMariaDb.MetricsKey(), &svcMonitor); err != nil {
					return false
				}

				g.Expect(svcMonitor.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(svcMonitor.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
				g.Expect(svcMonitor.ObjectMeta.Annotations).NotTo(BeNil())
				g.Expect(svcMonitor.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))

				g.Expect(svcMonitor.Spec.Selector.MatchLabels).NotTo(BeEmpty())
				g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "exporter"))
				g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", testMariaDb.MetricsKey().Name))
				g.Expect(svcMonitor.Spec.Endpoints).To(HaveLen(1))
				return true
			}).WithTimeout(testTimeout).WithPolling(testInterval).Should(BeTrue())
		})

		It("Should bootstrap from Backup", func() {
			backupKey := types.NamespacedName{
				Name:      "backup-mdb-from-backup",
				Namespace: testNamespace,
			}
			backup := testBackupWithS3Storage(backupKey, "test-mariadb", "")

			By("Creating Backup")
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			key := types.NamespacedName{
				Name:      "mariadb-from-backup",
				Namespace: testNamespace,
			}
			restoreSource := mariadbv1alpha1.RestoreSource{
				BackupRef: &corev1.LocalObjectReference{
					Name: backupKey.Name,
				},
				TargetRecoveryTime: &metav1.Time{Time: time.Now()},
			}
			testMariadbBootstrap(key, restoreSource)
		})

		It("Should bootstrap from S3", func() {
			backupKey := types.NamespacedName{
				Name:      "backup-mdb-from-s3",
				Namespace: testNamespace,
			}
			backup := testBackupWithS3Storage(backupKey, "test-mariadb", "s3")

			By("Creating Backup")
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			key := types.NamespacedName{
				Name:      "mariadb-from-s3",
				Namespace: testNamespace,
			}
			restoreSource := mariadbv1alpha1.RestoreSource{
				S3:                 testS3WithBucket("test-mariadb", "s3"),
				TargetRecoveryTime: &metav1.Time{Time: time.Now()},
			}
			testMariadbBootstrap(key, restoreSource)
		})
	})

	Context("When creating a MariaDB with generated passwords", func() {
		It("Should reconcile", func() {
			mdbKey := types.NamespacedName{
				Name:      "mariadb-generate",
				Namespace: testNamespace,
			}
			rootPasswordKey := types.NamespacedName{
				Name:      "mariadb-root-generate",
				Namespace: testNamespace,
			}

			mdb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mdbKey.Name,
					Namespace: mdbKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: rootPasswordKey.Name,
							},
							Key: testPwdSecretKey,
						},
						Generate: true,
					},
					Username: ptr.To("user"),
					PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
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

			By("Creating MariaDB")
			Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&mdb)
			})

			By("Expecting MariaDB to be ready eventually")
			expectMariadbReady(testCtx, k8sClient, mdbKey)

			By("Expecting to create a root Secret eventually")
			expectSecretToExist(testCtx, k8sClient, rootPasswordKey, testPwdSecretKey)

			By("Expecting to create a password Secret eventually")
			expectSecretToExist(testCtx, k8sClient, testPwdKey, testPwdSecretKey)
		})
	})

	Context("When updating a MariaDB", func() {
		It("Should reconcile", func() {
			By("Creating MariaDB")
			updateMariaDBKey := types.NamespacedName{
				Name:      "test-update-mariadb",
				Namespace: testNamespace,
			}
			updateMariaDB := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      updateMariaDBKey.Name,
					Namespace: updateMariaDBKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &updateMariaDB)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&updateMariaDB)
			})

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				return updateMariaDB.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Updating MariaDB image")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				updateMariaDB.Spec.Image = "mariadb:lts"
				return k8sClient.Update(testCtx, &updateMariaDB) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting image to be updated in StatefulSet eventually")
			Eventually(func() bool {
				var sts appsv1.StatefulSet
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &sts); err != nil {
					return false
				}
				return sts.Spec.Template.Spec.Containers[0].Image == "mariadb:lts"
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				return updateMariaDB.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())
		})
	})
})

var _ = Describe("MariaDB replication", func() {
	Context("When creating a MariaDB with replication", func() {
		It("Should reconcile", func() {
			mdb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-repl",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Username: &testUser,
					PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: testPwdKey.Name,
							},
							Key: testPwdSecretKey,
						},
					},
					Database: &testDatabase,
					MyCnf: func() *string {
						cfg := `[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M`
						return &cfg
					}(),
					Replication: &mariadbv1alpha1.Replication{
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Primary: &mariadbv1alpha1.PrimaryReplication{
								PodIndex:          func() *int { i := 0; return &i }(),
								AutomaticFailover: func() *bool { f := true; return &f }(),
							},
							Replica: &mariadbv1alpha1.ReplicaReplication{
								WaitPoint: func() *mariadbv1alpha1.WaitPoint { w := mariadbv1alpha1.WaitPointAfterSync; return &w }(),
								Gtid:      func() *mariadbv1alpha1.Gtid { g := mariadbv1alpha1.GtidCurrentPos; return &g }(),
							},
							SyncBinlog: func() *bool { s := true; return &s }(),
						},
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
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.120",
							},
						},
					},
					Connection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "mdb-repl-conn"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.130",
							},
						},
					},
					PrimaryConnection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "mdb-repl-conn-primary"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					SecondaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.131",
							},
						},
					},
					SecondaryConnection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "mdb-repl-conn-secondary"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
				},
			}

			By("Creating MariaDB with replication")
			Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&mdb)
			})

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &svc)).To(Succeed())

			By("Expecting to create a primary Service")
			Expect(k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc)).To(Succeed())
			Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(mdb.ObjectMeta, 0)))

			By("Expecting to create a secondary Service")
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &svc)).To(Succeed())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, mdb.PrimaryConnectioneKey(), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting secondary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, mdb.SecondaryConnectioneKey(), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create secondary Endpoints")
			var endpoints corev1.Endpoints
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			Expect(endpoints.Subsets).To(HaveLen(1))
			Expect(endpoints.Subsets[0].Addresses).To(HaveLen(int(mdb.Spec.Replicas) - 1))

			By("Expecting to create a PodDisruptionBudget")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &pdb)).To(Succeed())

			By("Expecting MariaDB to eventually update primary")
			podIndex := 1
			Eventually(func(g Gomega) bool {
				g.Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb)).To(Succeed())
				mdb.Replication().Primary.PodIndex = &podIndex
				g.Expect(k8sClient.Update(testCtx, &mdb)).To(Succeed())
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				if !mdb.IsReady() || mdb.Status.CurrentPrimaryPodIndex == nil {
					return false
				}
				return *mdb.Status.CurrentPrimaryPodIndex == podIndex
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Service to eventually change primary")
			Eventually(func() bool {
				var svc corev1.Service
				if err := k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc); err != nil {
					return false
				}
				return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == statefulset.PodName(mdb.ObjectMeta, podIndex)
			}, testTimeout, testInterval).Should(BeTrue())

			By("Tearing down primary Pod consistently")
			Consistently(func() bool {
				primaryPodKey := types.NamespacedName{
					Name:      statefulset.PodName(mdb.ObjectMeta, 1),
					Namespace: mdb.Namespace,
				}
				var primaryPod corev1.Pod
				if err := k8sClient.Get(testCtx, primaryPodKey, &primaryPod); err != nil {
					return apierrors.IsNotFound(err)
				}
				return k8sClient.Delete(testCtx, &primaryPod) == nil
			}, 10*time.Second, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				if !mdb.IsReady() || mdb.Status.CurrentPrimaryPodIndex == nil {
					return false
				}
				return *mdb.Status.CurrentPrimaryPodIndex == 0 || *mdb.Status.CurrentPrimaryPodIndex == 2
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, mdb.PrimaryConnectioneKey(), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			mxsKey := types.NamespacedName{
				Name:      "maxscale-repl",
				Namespace: testNamespace,
			}
			expectMariadbMaxScaleReady(&mdb, mxsKey)
		})
	})
})

var _ = Describe("MariaDB Galera", func() {
	Context("When creating a MariaDB Galera", func() {
		It("Should default", func() {
			By("Creating MariaDB")
			testDefaultMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mariadb-galera-default",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					Replicas: 3,
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &testDefaultMariaDb)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&testDefaultMariaDb)
			})

			By("Expecting to eventually default")
			Eventually(func(g Gomega) bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testDefaultMariaDb), &testDefaultMariaDb); err != nil {
					return false
				}
				g.Expect(testDefaultMariaDb.Spec.Galera).ToNot(BeZero())
				g.Expect(testDefaultMariaDb.Spec.Galera.Primary).ToNot(BeZero())
				g.Expect(testDefaultMariaDb.Spec.Galera.SST).ToNot(BeZero())
				g.Expect(testDefaultMariaDb.Spec.Galera.ReplicaThreads).ToNot(BeZero())
				g.Expect(testDefaultMariaDb.Spec.Galera.Recovery).ToNot(BeZero())
				g.Expect(testDefaultMariaDb.Spec.Galera.InitContainer).ToNot(BeZero())
				g.Expect(testDefaultMariaDb.Spec.Galera.Config).ToNot(BeZero())
				return true
			}, testTimeout, testInterval).Should(BeTrue())
		})

		galeraKey := types.NamespacedName{
			Name:      "mariadb-galera",
			Namespace: testNamespace,
		}

		It("Should reconcile", Serial, func() {
			mdb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      galeraKey.Name,
					Namespace: galeraKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Username: &testUser,
					PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: testPwdKey.Name,
							},
							Key: testPwdSecretKey,
						},
					},
					Database: &testDatabase,
					MyCnf: ptr.To(`[mariadb]
					bind-address=*
					default_storage_engine=InnoDB
					binlog_format=row
					innodb_autoinc_lock_mode=2
					max_allowed_packet=256M
					`),
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Primary: mariadbv1alpha1.PrimaryGalera{
								PodIndex:          ptr.To(0),
								AutomaticFailover: ptr.To(true),
							},
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled: true,
							},
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(false),
								VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
									PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
										Resources: corev1.VolumeResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("100Mi"),
											},
										},
										AccessModes: []corev1.PersistentVolumeAccessMode{
											corev1.ReadWriteOnce,
										},
									},
								},
							},
							InitJob: &mariadbv1alpha1.Job{
								Metadata: &mariadbv1alpha1.Metadata{
									Labels: map[string]string{
										"sidecar.istio.io/inject": "false",
									},
								},
							},
						},
					},
					Replicas: 3,
					Storage: mariadbv1alpha1.Storage{
						Size: ptr.To(resource.MustParse("300Mi")),
					},
					Service: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.150",
							},
						},
					},
					Connection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: ptr.To("mdb-galera-conn"),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.160",
							},
						},
					},
					PrimaryConnection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: ptr.To("mdb-galera-conn-primary"),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					SecondaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.161",
							},
						},
					},
					SecondaryConnection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: ptr.To("mdb-galera-conn-secondary"),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
				},
			}

			By("Creating MariaDB Galera")
			Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&mdb)
			})

			By("Expecting init Job to eventually be completed")
			Eventually(func(g Gomega) bool {
				var job batchv1.Job
				g.Expect(k8sClient.Get(testCtx, mdb.InitKey(), &job)).To(Succeed())

				g.Expect(job.ObjectMeta.Labels).NotTo(BeNil())
				g.Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("sidecar.istio.io/inject", "false"))

				return jobpkg.IsJobComplete(&job)
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.IsReady() && mdb.HasGaleraConfiguredCondition() && mdb.HasGaleraReadyCondition()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a StatefulSet")
			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &sts)).To(Succeed())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &svc)).To(Succeed())

			By("Expecting to create a primary Service")
			Expect(k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc)).To(Succeed())
			Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(mdb.ObjectMeta, 0)))

			By("Expecting to create a secondary Service")
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &svc)).To(Succeed())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, mdb.PrimaryConnectioneKey(), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting secondary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, mdb.SecondaryConnectioneKey(), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create secondary Endpoints")
			var endpoints corev1.Endpoints
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			Expect(endpoints.Subsets).To(HaveLen(1))
			Expect(endpoints.Subsets[0].Addresses).To(HaveLen(int(mdb.Spec.Replicas) - 1))

			By("Expecting to create a PodDisruptionBudget")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &pdb)).To(Succeed())

			By("Updating MariaDB primary")
			podIndex := 1
			mdb.Spec.Galera.Primary.PodIndex = ptr.To(podIndex)
			Expect(k8sClient.Update(testCtx, &mdb)).To(Succeed())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				if !mdb.IsReady() || mdb.Status.CurrentPrimaryPodIndex == nil {
					return false
				}
				return *mdb.Status.CurrentPrimaryPodIndex == podIndex
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Service to eventually change primary")
			Eventually(func() bool {
				var svc corev1.Service
				if err := k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc); err != nil {
					return false
				}
				return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == statefulset.PodName(mdb.ObjectMeta, podIndex)
			}, testTimeout, testInterval).Should(BeTrue())

			By("Tearing down all Pods consistently")
			opts := []client.DeleteAllOfOption{
				client.MatchingLabels{
					"app.kubernetes.io/instance": mdb.Name,
				},
				client.InNamespace(mdb.Namespace),
			}
			Expect(k8sClient.DeleteAllOf(testCtx, &corev1.Pod{}, opts...)).To(Succeed())

			By("Expecting MariaDB NOT to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return !mdb.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Galera NOT to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.HasGaleraNotReadyCondition()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.IsReady() && mdb.HasGaleraReadyCondition()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, mdb.PrimaryConnectioneKey(), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			mxsKey := types.NamespacedName{
				Name:      "maxscale-galera",
				Namespace: testNamespace,
			}
			expectMariadbMaxScaleReady(&mdb, mxsKey)
		})

		It("Should reconcile storage", Serial, func() {
			mdb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      galeraKey.Name,
					Namespace: galeraKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(true),
							},
						},
					},
					Replicas: 3,
					Storage: mariadbv1alpha1.Storage{
						Size:                ptr.To(resource.MustParse("300Mi")),
						StorageClassName:    "standard-resize",
						ResizeInUseVolumes:  ptr.To(true),
						WaitForVolumeResize: ptr.To(true),
					},
					Service: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Metadata: &mariadbv1alpha1.Metadata{
							Annotations: map[string]string{
								"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.150",
							},
						},
					},
				},
			}

			By("Creating MariaDB Galera")
			Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
			DeferCleanup(func() {
				deleteMariaDB(&mdb)
			})

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.IsReady() && mdb.HasGaleraConfiguredCondition() && mdb.HasGaleraReadyCondition()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Updating storage")
			mdb.Spec.Storage.Size = ptr.To(resource.MustParse("400Mi"))
			Expect(k8sClient.Update(testCtx, &mdb)).To(Succeed())

			By("Expecting MariaDB to have resized storage eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &mdb); err != nil {
					return false
				}
				return mdb.IsReady() && meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeStorageResized)
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting StatefulSet storage to have been resized")
			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mdb), &sts)).To(Succeed())
			mdbSize := mdb.Spec.Storage.GetSize()
			stsSize := stsobj.GetStorageSize(&sts, builder.StorageVolume)
			Expect(mdbSize).NotTo(BeNil())
			Expect(stsSize).NotTo(BeNil())
			Expect(mdbSize.Cmp(*stsSize)).To(Equal(0))

			By("Expecting PVCs to have been resized")
			pvcList := corev1.PersistentVolumeClaimList{}
			listOpts := client.ListOptions{
				LabelSelector: klabels.SelectorFromSet(
					labels.NewLabelsBuilder().
						WithMariaDBSelectorLabels(&mdb).
						WithPVCRole(builder.StorageVolumeRole).
						Build(),
				),
				Namespace: mdb.GetNamespace(),
			}
			Expect(k8sClient.List(testCtx, &pvcList, &listOpts)).To(Succeed())
			for _, p := range pvcList.Items {
				pvcSize := p.Spec.Resources.Requests[corev1.ResourceStorage]
				Expect(mdbSize.Cmp(pvcSize)).To(Equal(0))
			}
		})
	})
})

func expectMariadbMaxScaleReady(mdb *mariadbv1alpha1.MariaDB, mxsKey types.NamespacedName) {
	mxs := mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mxsKey.Name,
			Namespace: mxsKey.Namespace,
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			MariaDBRef: &mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name: client.ObjectKeyFromObject(mdb).Name,
				},
			},
		},
	}
	By("Creating MaxScale")
	Expect(k8sClient.Create(testCtx, &mxs)).To(Succeed())
	DeferCleanup(func() {
		deleteMaxScale(mxsKey, true)
	})

	By("Point MariaDB to MaxScale")
	mdb.Spec.MaxScaleRef = &corev1.ObjectReference{
		Name:      mxsKey.Name,
		Namespace: mxsKey.Namespace,
	}
	Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())

	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(mdb), mdb); err != nil {
			return false
		}
		return mdb.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting MaxScale to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, mxsKey, &mxs); err != nil {
			return false
		}
		return mxs.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func deploymentReady(deploy *appsv1.Deployment) bool {
	for _, c := range deploy.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func deleteMariaDB(mdb *mariadbv1alpha1.MariaDB) {
	Expect(k8sClient.Delete(testCtx, mdb)).To(Succeed())

	Eventually(func(g Gomega) bool {
		listOpts := &client.ListOptions{
			LabelSelector: klabels.SelectorFromSet(
				labels.NewLabelsBuilder().
					WithMariaDBSelectorLabels(mdb).
					Build(),
			),
			Namespace: mdb.GetNamespace(),
		}
		pvcList := &corev1.PersistentVolumeClaimList{}
		g.Expect(k8sClient.List(testCtx, pvcList, listOpts)).To(Succeed())

		for _, pvc := range pvcList.Items {
			g.Expect(k8sClient.Delete(testCtx, &pvc)).To(Succeed())
		}
		return true
	}, 30*time.Second, 1*time.Second).Should(BeTrue())
}

func testMariadbBootstrap(mdbKey types.NamespacedName, source mariadbv1alpha1.RestoreSource) {
	mdb := mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mdbKey.Name,
			Namespace: mdbKey.Namespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
				RestoreSource: source,
				RestoreJob: &mariadbv1alpha1.Job{
					Metadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
					},
				},
			},
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("100Mi")),
			},
		},
	}

	By("Creating a MariaDB bootstrapping from backup")
	Expect(k8sClient.Create(testCtx, &mdb)).To(Succeed())
	DeferCleanup(func() {
		deleteMariaDB(&mdb)
	})

	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, mdbKey, &mdb); err != nil {
			return false
		}
		return mdb.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting MariaDB to eventually have restored backup")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, mdbKey, &mdb); err != nil {
			return false
		}
		return mdb.HasRestoredBackup()
	}, testTimeout, testInterval).Should(BeTrue())
}
