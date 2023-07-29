package controllers

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controllers/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB", func() {
	Context("When creating a MariaDB", func() {
		It("Should reconcile", func() {
			By("Expecting to have spec provided by user and defaults")
			Expect(testMariaDb.Spec.Image.String()).To(Equal("mariadb:11.0.2"))
			Expect(testMariaDb.Spec.Port).To(BeEquivalentTo(3306))

			By("Expecting to create a ConfigMap eventually")
			Eventually(func() bool {
				var cm corev1.ConfigMap
				if err := k8sClient.Get(testCtx, configMapMariaDBKey(&testMariaDb), &cm); err != nil {
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
				Expect(svc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/instance", testMariaDbName))
				Expect(svc.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "mariadb"))
				Expect(svc.ObjectMeta.Annotations).NotTo(BeNil())
				return true
			}, testTimeout, testInterval).Should(BeTrue())

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
						LocalObjectReference: corev1.LocalObjectReference{
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
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					BootstrapFrom: &mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: backupKey.Name,
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					BootstrapFrom: &mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
			By("Performing update")
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
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
			updateMariaDB.Spec.Port = 3307
			Expect(k8sClient.Update(testCtx, &updateMariaDB)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				return updateMariaDB.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting port to be updated in StatefulSet")
			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(testCtx, updateMariaDBKey, &sts)).To(Succeed())
			containerPort, err := builder.StatefulSetPort(&sts)
			Expect(err).NotTo(HaveOccurred())
			Expect(containerPort.ContainerPort).To(BeEquivalentTo(3307))

			By("Expecting port to be updated in Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, updateMariaDBKey, &svc)).To(Succeed())
			svcPort, err := builder.MariaDBPort(&svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svcPort.Port).To(BeEquivalentTo(3307))

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &updateMariaDB)).To(Succeed())
		})
	})
})

var _ = Describe("MariaDB replication", func() {
	Context("When creating a MariaDB with replication", func() {
		It("Should reconcile and switch primary", func() {
			testRplMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-repl",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Username: &testUser,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
					Connection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "conn-mdb-repl"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
						bind-address=0.0.0.0
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M`
						return &cfg
					}(),
					Replication: &mariadbv1alpha1.Replication{
						Primary: mariadbv1alpha1.PrimaryReplication{
							PodIndex:          0,
							AutomaticFailover: true,
							Service: &mariadbv1alpha1.Service{
								Type: corev1.ServiceTypeLoadBalancer,
								Annotations: map[string]string{
									"metallb.universe.tf/loadBalancerIPs": "172.18.0.130",
								},
							},
							Connection: &mariadbv1alpha1.ConnectionTemplate{
								SecretName: func() *string {
									s := "primary-conn-mdb-repl"
									return &s
								}(),
								SecretTemplate: &mariadbv1alpha1.SecretTemplate{
									Key: &testConnSecretKey,
								},
							},
						},
						Replica: mariadbv1alpha1.ReplicaReplication{
							WaitPoint: func() *mariadbv1alpha1.WaitPoint { w := mariadbv1alpha1.WaitPointAfterSync; return &w }(),
							Gtid:      func() *mariadbv1alpha1.Gtid { g := mariadbv1alpha1.GtidCurrentPos; return &g }(),
						},
						SyncBinlog: true,
					},
					Replicas: 3,
					Service: &mariadbv1alpha1.Service{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": "172.18.0.120",
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
			}, 90*time.Second, testInterval).Should(BeTrue())

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

			By("Expecting to create a PodDisruptionBudget")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &pdb)).To(Succeed())

			By("Expecting to create a primary Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, ctrlresources.PrimaryServiceKey(&testRplMariaDb), &svc)).To(Succeed())

			By("Updating MariaDB")
			testRplMariaDb.Spec.Replication.Primary.PodIndex = 1
			Expect(k8sClient.Update(testCtx, &testRplMariaDb)).To(Succeed())

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				if !testRplMariaDb.IsReady() {
					return false
				}
				if testRplMariaDb.Status.CurrentPrimaryPodIndex != nil {
					return *testRplMariaDb.Status.CurrentPrimaryPodIndex == 1
				}
				return false
			}, testTimeout, testInterval).Should(BeTrue())

			By("Tearing down primary Pod")
			primaryPodKey := types.NamespacedName{
				Name:      statefulset.PodName(testRplMariaDb.ObjectMeta, 1),
				Namespace: testRplMariaDb.Namespace,
			}
			var primaryPod corev1.Pod
			Expect(k8sClient.Get(testCtx, primaryPodKey, &primaryPod))
			Expect(k8sClient.Delete(testCtx, &primaryPod))

			By("Expecting MariaDB to eventually change primary")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				if !testRplMariaDb.IsReady() {
					return false
				}
				if testRplMariaDb.Status.CurrentPrimaryPodIndex != nil {
					return *testRplMariaDb.Status.CurrentPrimaryPodIndex == 0 || *testRplMariaDb.Status.CurrentPrimaryPodIndex == 2
				}
				return false
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testRplMariaDb), &testRplMariaDb); err != nil {
					return false
				}
				return testRplMariaDb.IsReady()
			}, 90*time.Second, testInterval).Should(BeTrue())

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
		It("Should reconcile and recover cluster", func() {
			timeout := 5 * time.Minute
			clusterHealthyTimeout := metav1.Duration{Duration: 30 * time.Second}
			recoveryTimeout := metav1.Duration{Duration: timeout}
			testMariaDbGalera := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-galera",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Image: mariadbv1alpha1.Image{
							Repository: "mariadb",
							Tag:        "11.0.2",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Username: &testUser,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
					Connection: &mariadbv1alpha1.ConnectionTemplate{
						SecretName: func() *string {
							s := "conn-mdb-galera"
							return &s
						}(),
						SecretTemplate: &mariadbv1alpha1.SecretTemplate{
							Key: &testConnSecretKey,
						},
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
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
						bind-address=0.0.0.0
						default_storage_engine=InnoDB
						binlog_format=row
						innodb_autoinc_lock_mode=2
						max_allowed_packet=256M`
						return &cfg
					}(),
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Recovery: &mariadbv1alpha1.GaleraRecovery{
								Enabled:                 true,
								ClusterHealthyTimeout:   &clusterHealthyTimeout,
								ClusterBootstrapTimeout: &recoveryTimeout,
								PodRecoveryTimeout:      &recoveryTimeout,
								PodSyncTimeout:          &recoveryTimeout,
							},
							VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
								Spec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("10Mi"),
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
					Service: &mariadbv1alpha1.Service{
						Type: corev1.ServiceTypeLoadBalancer,
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": "172.18.0.150",
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
			}, timeout, testInterval).Should(BeTrue())
			By("Expecting Galera to be configured eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraConfiguredCondition()
			}, timeout, testInterval).Should(BeTrue())
			By("Expecting Galera to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraReadyCondition()
			}, timeout, testInterval).Should(BeTrue())
			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, timeout, testInterval).Should(BeTrue())

			By("Expecting to create a PodDisruptionBudget")
			var pdb policyv1.PodDisruptionBudget
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &pdb)).To(Succeed())

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
			}, timeout, testInterval).Should(BeTrue())
			By("Expecting Galera NOT to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraNotReadyCondition()
			}, timeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.IsReady()
			}, timeout, testInterval).Should(BeTrue())
			By("Expecting Galera to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &testMariaDbGalera); err != nil {
					return false
				}
				return testMariaDbGalera.HasGaleraReadyCondition()
			}, timeout, testInterval).Should(BeTrue())
			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&testMariaDbGalera), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, timeout, testInterval).Should(BeTrue())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &testMariaDbGalera)).To(Succeed())
		})
	})
})
