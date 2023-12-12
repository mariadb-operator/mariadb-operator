package controller

import (
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controller/resource"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB", func() {
	Context("When creating a MariaDB", func() {
		It("Should default", func() {
			By("Creating MariaDB")
			testDefaultKey := types.NamespacedName{
				Name:      "test-default",
				Namespace: testNamespace,
			}
			testDefaultMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testDefaultKey.Name,
					Namespace: testDefaultKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					MyCnf: ptr.To(`
					[mariadb]
					bind-address=*
					default_storage_engine=InnoDB
					binlog_format=row
					innodb_autoinc_lock_mode=2
					max_allowed_packet=256M
					`),
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
				},
			}
			Expect(k8sClient.Create(testCtx, &testDefaultMariaDb)).To(Succeed())

			By("Expecting to eventually default")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, testDefaultKey, &testDefaultMariaDb); err != nil {
					return false
				}
				return testDefaultMariaDb.Spec.Image != ""
			}, testTimeout, testInterval).Should(BeTrue())

			expectedImage := os.Getenv("RELATED_IMAGE_MARIADB")
			Expect(expectedImage).ToNot((BeEmpty()))
			Expect(testDefaultMariaDb.Spec.Image).To(Equal(expectedImage))
			Expect(testDefaultMariaDb.Spec.RootPasswordSecretKeyRef).To(BeEquivalentTo(testDefaultMariaDb.RootPasswordSecretKeyRef()))
			Expect(testDefaultMariaDb.Spec.Port).To(BeEquivalentTo(3306))
			Expect(testDefaultMariaDb.Spec.MyCnfConfigMapKeyRef).ToNot(BeNil())
		})
		It("Should reconcile", func() {
			By("Expecting to create a ConfigMap eventually")
			Eventually(func() bool {
				var cm corev1.ConfigMap
				key := types.NamespacedName{
					Name:      testMariaDb.MyCnfConfigMapKeyRef().Name,
					Namespace: testMariaDb.Namespace,
				}
				if err := k8sClient.Get(testCtx, key, &cm); err != nil {
					return false
				}
				Expect(cm.ObjectMeta.Labels).NotTo(BeNil())
				Expect(cm.ObjectMeta.Annotations).NotTo(BeNil())
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a StatefulSet eventually")
			Eventually(func() bool {
				var sts appsv1.StatefulSet
				if err := k8sClient.Get(testCtx, testMariaDbKey, &sts); err != nil {
					return false
				}
				Expect(sts.ObjectMeta.Labels).NotTo(BeNil())
				Expect(sts.ObjectMeta.Annotations).NotTo(BeNil())
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service eventually")
			Eventually(func() bool {
				var svc corev1.Service
				if err := k8sClient.Get(testCtx, testMariaDbKey, &svc); err != nil {
					return false
				}
				Expect(svc.ObjectMeta.Labels).NotTo(BeNil())
				Expect(svc.ObjectMeta.Labels).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))
				Expect(svc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "mariadb"))
				Expect(svc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/instance", testMariaDbName))
				Expect(svc.ObjectMeta.Annotations).NotTo(BeNil())
				Expect(svc.ObjectMeta.Annotations).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a ServiceMonitor eventually")
			Eventually(func() bool {
				var svcMonitor monitoringv1.ServiceMonitor
				if err := k8sClient.Get(testCtx, testMariaDbKey, &svcMonitor); err != nil {
					return false
				}
				Expect(svcMonitor.Spec.Selector).NotTo(BeNil())
				Expect(svcMonitor.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", "mariadb"))
				Expect(svcMonitor.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/instance", testMariaDbName))
				return true
			})

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDb), &conn); err != nil {
					return false
				}
				Expect(conn.ObjectMeta.Labels).NotTo(BeNil())
				Expect(conn.ObjectMeta.Annotations).NotTo(BeNil())
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())
		})

		It("Should bootstrap from backup", func() {
			By("Creating Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-mariadb-test",
				Namespace: testNamespace,
			}
			backup := mariadbv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
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
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			By("Expecting Backup to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating a MariaDB bootstrapping from backup")
			bootstrapMariaDBKey := types.NamespacedName{
				Name:      "mariadb-backup",
				Namespace: testNamespace,
			}
			bootstrapMariaDB := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapMariaDBKey.Name,
					Namespace: bootstrapMariaDBKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: backupKey.Name,
						},
					},
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
				},
			}
			Expect(k8sClient.Create(testCtx, &bootstrapMariaDB)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, bootstrapMariaDBKey, &bootstrapMariaDB); err != nil {
					return false
				}
				return bootstrapMariaDB.IsReady()
			}, 60*time.Second, testInterval).Should(BeTrue())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &bootstrapMariaDB)).To(Succeed())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, &backup)).To(Succeed())
		})
	})

	Context("When creating an invalid MariaDB", func() {
		It("Should report not ready status", func() {
			By("Creating MariaDB")
			invalidMariaDbKey := types.NamespacedName{
				Name:      "mariadb-test-invalid",
				Namespace: testNamespace,
			}
			invalidMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      invalidMariaDbKey.Name,
					Namespace: invalidMariaDbKey.Namespace,
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
				},
			}
			Expect(k8sClient.Create(testCtx, &invalidMariaDb)).To(Succeed())

			By("Expecting not ready status consistently")
			Consistently(func() bool {
				if err := k8sClient.Get(testCtx, invalidMariaDbKey, &invalidMariaDb); err != nil {
					return false
				}
				return !invalidMariaDb.IsReady()
			}, 5*time.Second, testInterval)

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &invalidMariaDb)).To(Succeed())
		})
	})

	Context("When bootstrapping from a non existing backup", func() {
		It("Should report not ready status", func() {
			By("Creating MariaDB")
			noBackupKey := types.NamespacedName{
				Name:      "mariadb-test-no-backup",
				Namespace: testNamespace,
			}
			noBackup := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noBackupKey.Name,
					Namespace: noBackupKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
					},
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
				},
			}
			Expect(k8sClient.Create(testCtx, &noBackup)).To(Succeed())

			By("Expecting not ready status consistently")
			Consistently(func() bool {
				if err := k8sClient.Get(testCtx, noBackupKey, &noBackup); err != nil {
					return false
				}
				return !noBackup.IsReady()
			}, 5*time.Second, testInterval)

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &noBackup)).To(Succeed())
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
				},
			}
			Expect(k8sClient.Create(testCtx, &updateMariaDB)).To(Succeed())

			By("Updating MariaDB image")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				updateMariaDB.Spec.Image = "mariadb:lts"
				return k8sClient.Update(testCtx, &updateMariaDB) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				return updateMariaDB.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting image to be updated in StatefulSet")
			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(testCtx, updateMariaDBKey, &sts)).To(Succeed())
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(BeEquivalentTo("mariadb:lts"))

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &updateMariaDB)).To(Succeed())
		})
	})
})

var _ = Describe("MariaDB replication", func() {
	Context("When creating a MariaDB with replication", func() {
		It("Should reconcile", func() {
			testRplMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-repl",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Username: &testUser,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
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
					Service: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.120",
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
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.130",
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
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.131",
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
			Expect(k8sClient.Create(testCtx, &testRplMariaDb)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				return testRplMariaDb.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &svc)).To(Succeed())

			By("Expecting to create a primary Service")
			Expect(k8sClient.Get(testCtx, ctrlresources.PrimaryServiceKey(&testRplMariaDb), &svc)).To(Succeed())
			Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(testRplMariaDb.ObjectMeta, 0)))

			By("Expecting to create a secondary Service")
			Expect(k8sClient.Get(testCtx, ctrlresources.SecondaryServiceKey(&testRplMariaDb), &svc)).To(Succeed())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, ctrlresources.PrimaryConnectioneKey(&testRplMariaDb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting secondary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, ctrlresources.SecondaryConnectioneKey(&testRplMariaDb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create secondary Endpoints")
			var endpoints corev1.Endpoints
			Expect(k8sClient.Get(testCtx, ctrlresources.SecondaryServiceKey(&testRplMariaDb), &endpoints)).To(Succeed())
			Expect(endpoints.Subsets).To(HaveLen(1))
			Expect(endpoints.Subsets[0].Addresses).To(HaveLen(int(testRplMariaDb.Spec.Replicas) - 1))

			By("Expecting to create a PodDisruptionBudget")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &pdb)).To(Succeed())

			By("Updating MariaDB primary")
			podIndex := 1
			testRplMariaDb.Replication().Primary.PodIndex = &podIndex
			Expect(k8sClient.Update(testCtx, &testRplMariaDb)).To(Succeed())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				if !testRplMariaDb.IsReady() || testRplMariaDb.Status.CurrentPrimaryPodIndex == nil {
					return false
				}
				return *testRplMariaDb.Status.CurrentPrimaryPodIndex == podIndex
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Service to eventually change primary")
			Eventually(func() bool {
				var svc corev1.Service
				if err := k8sClient.Get(testCtx, ctrlresources.PrimaryServiceKey(&testRplMariaDb), &svc); err != nil {
					return false
				}
				return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == statefulset.PodName(testRplMariaDb.ObjectMeta, podIndex)
			}, testTimeout, testInterval).Should(BeTrue())

			By("Tearing down primary Pod")
			primaryPodKey := types.NamespacedName{
				Name:      statefulset.PodName(testRplMariaDb.ObjectMeta, 1),
				Namespace: testRplMariaDb.Namespace,
			}
			var primaryPod corev1.Pod
			Expect(k8sClient.Get(testCtx, primaryPodKey, &primaryPod))
			Expect(k8sClient.Delete(testCtx, &primaryPod))

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				return testRplMariaDb.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				if !testRplMariaDb.IsReady() || testRplMariaDb.Status.CurrentPrimaryPodIndex == nil {
					return false
				}
				return *testRplMariaDb.Status.CurrentPrimaryPodIndex == 0 || *testRplMariaDb.Status.CurrentPrimaryPodIndex == 2
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, ctrlresources.PrimaryConnectioneKey(&testRplMariaDb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &testRplMariaDb)).To(Succeed())
		})
	})
})

var _ = Describe("MariaDB Galera", func() {
	Context("When creating a MariaDB Galera", func() {
		It("Should reconcile", func() {
			clusterHealthyTimeout := metav1.Duration{Duration: 30 * time.Second}
			recoveryTimeout := metav1.Duration{Duration: 5 * time.Minute}
			testMariaDbGalera := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Username: &testUser,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
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
					MyCnf: func() *string {
						cfg := `[mariadb]
						bind-address=*
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M`
						return &cfg
					}(),
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Primary: &mariadbv1alpha1.PrimaryGalera{
								PodIndex:          func() *int { i := 0; return &i }(),
								AutomaticFailover: func() *bool { af := true; return &af }(),
							},
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled:                 true,
								ClusterHealthyTimeout:   &clusterHealthyTimeout,
								ClusterBootstrapTimeout: &recoveryTimeout,
								PodRecoveryTimeout:      &recoveryTimeout,
								PodSyncTimeout:          &recoveryTimeout,
							},
							VolumeClaimTemplate: &mariadbv1alpha1.VolumeClaimTemplate{
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
						},
					},
					Replicas: 3,
					Service: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.150",
						},
					},
					Connection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "mdb-galera-conn"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					PrimaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.160",
						},
					},
					PrimaryConnection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "mdb-galera-conn-primary"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					SecondaryService: &mariadbv1alpha1.ServiceTemplate{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.161",
						},
					},
					SecondaryConnection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "mdb-galera-conn-secondary"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
				},
			}

			By("Creating MariaDB Galera")
			Expect(k8sClient.Create(testCtx, &testMariaDbGalera)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.IsReady()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Galera to be configured eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraConfiguredCondition()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting Galera to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraReadyCondition()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &svc)).To(Succeed())

			By("Expecting to create a primary Service")
			Expect(k8sClient.Get(testCtx, ctrlresources.PrimaryServiceKey(&testMariaDbGalera), &svc)).To(Succeed())
			Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(testMariaDbGalera.ObjectMeta, 0)))

			By("Expecting to create a secondary Service")
			Expect(k8sClient.Get(testCtx, ctrlresources.SecondaryServiceKey(&testMariaDbGalera), &svc)).To(Succeed())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, ctrlresources.PrimaryConnectioneKey(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting secondary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, ctrlresources.SecondaryConnectioneKey(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create secondary Endpoints")
			var endpoints corev1.Endpoints
			Expect(k8sClient.Get(testCtx, ctrlresources.SecondaryServiceKey(&testMariaDbGalera), &endpoints)).To(Succeed())
			Expect(endpoints.Subsets).To(HaveLen(1))
			Expect(endpoints.Subsets[0].Addresses).To(HaveLen(int(testMariaDbGalera.Spec.Replicas) - 1))

			By("Expecting to create a PodDisruptionBudget")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &pdb)).To(Succeed())

			By("Updating MariaDB primary")
			podIndex := 1
			testMariaDbGalera.Galera().Primary.PodIndex = &podIndex
			Expect(k8sClient.Update(testCtx, &testMariaDbGalera)).To(Succeed())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				if !testMariaDbGalera.IsReady() || testMariaDbGalera.Status.CurrentPrimaryPodIndex == nil {
					return false
				}
				return *testMariaDbGalera.Status.CurrentPrimaryPodIndex == podIndex
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Service to eventually change primary")
			Eventually(func() bool {
				var svc corev1.Service
				if err := k8sClient.Get(testCtx, ctrlresources.PrimaryServiceKey(&testMariaDbGalera), &svc); err != nil {
					return false
				}
				return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == statefulset.PodName(testMariaDbGalera.ObjectMeta, podIndex)
			}, testTimeout, testInterval).Should(BeTrue())

			By("Tearing down Pods")
			opts := []client.DeleteAllOfOption{
				client.MatchingLabels{
					"app.kubernetes.io/instance": testMariaDbGalera.Name,
				},
				client.InNamespace(testMariaDbGalera.Namespace),
			}
			Expect(k8sClient.DeleteAllOf(testCtx, &corev1.Pod{}, opts...)).To(Succeed())

			By("Expecting MariaDB NOT to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.IsReady()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Galera NOT to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraNotReadyCondition()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.IsReady()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Galera to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraReadyCondition()
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting primary Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, ctrlresources.PrimaryConnectioneKey(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &testMariaDbGalera)).To(Succeed())
		})
	})
})
