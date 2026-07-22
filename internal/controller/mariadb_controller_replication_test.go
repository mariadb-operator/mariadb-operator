package controller

import (
	"fmt"
	"strconv"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB replication", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	BeforeAll(func() {
		mdb = buildTestMariaDBWithRepl(key)
		applyMariadbTestConfig(mdb)

		By("Creating MariaDB with replication")
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})
	})

	It("should reconcile", Label("basic"), func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Service")
		var svc corev1.Service
		Expect(k8sClient.Get(testCtx, key, &svc)).To(Succeed())

		By("Expecting to create a primary Service")
		Expect(k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc)).To(Succeed())
		Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(stsobj.PodName(mdb.ObjectMeta, 0)))

		By("Expecting to create a secondary Service")
		Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &svc)).To(Succeed())

		By("Expecting role label to be set to primary")
		Eventually(func() bool {
			currentPrimary := *mdb.Status.CurrentPrimary
			primaryPodKey := types.NamespacedName{
				Name:      currentPrimary,
				Namespace: mdb.Namespace,
			}
			var primaryPod corev1.Pod
			if err := k8sClient.Get(testCtx, primaryPodKey, &primaryPod); err != nil {
				return apierrors.IsNotFound(err)
			}
			return primaryPod.Labels["k8s.mariadb.com/role"] == "primary"
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Connection to be ready eventually")
		Eventually(func() bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
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
		var endpointSlice discoveryv1.EndpointSlice
		Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpointSlice)).To(Succeed())
		Expect(endpointSlice.Ports).To(HaveLen(1))
		Expect(endpointSlice.Ports[0].Port).ToNot(BeNil())
		Expect(*endpointSlice.Ports[0].Port).To(BeEquivalentTo(mdb.Spec.Port))
		Expect(endpointSlice.Endpoints).To(HaveLen(int(mdb.Spec.Replicas) - 1))
		Expect(endpointSlice.Endpoints[0].Addresses).To(HaveLen(int(1)))

		By("Expecting to create a PodDisruptionBudget")
		var pdb policyv1.PodDisruptionBudget
		Expect(k8sClient.Get(testCtx, key, &pdb)).To(Succeed())
	})

	It("should fail and switch over primary", func() {
		By("Expecting MariaDB primary to be set")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.Status.CurrentPrimary != nil
		}, testTimeout, testInterval).Should(BeTrue())

		var currentPrimary string
		By("Tearing down primary Pod")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			currentPrimary = *mdb.Status.CurrentPrimary
			primaryPodKey := types.NamespacedName{
				Name:      *mdb.Status.CurrentPrimary,
				Namespace: mdb.Namespace,
			}
			var primaryPod corev1.Pod
			if err := k8sClient.Get(testCtx, primaryPodKey, &primaryPod); err != nil {
				return apierrors.IsNotFound(err)
			}
			return k8sClient.Delete(testCtx, &primaryPod, &client.DeleteOptions{
				GracePeriodSeconds: ptr.To(int64(0)),
				PropagationPolicy:  ptr.To(metav1.DeletePropagationForeground),
			}) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually change primary")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			if !mdb.IsReady() || mdb.Status.CurrentPrimary == nil {
				return false
			}
			return *mdb.Status.CurrentPrimary != currentPrimary
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting Connection to be ready eventually")
		Eventually(func() bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
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

		By("Expecting MariaDB to eventually update primary")
		var podIndex int
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			if i != *mdb.Status.CurrentPrimaryPodIndex {
				podIndex = i
				break
			}
		}
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			mdb.Spec.Replication.Primary.PodIndex = &podIndex
			g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually change primary")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
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
			return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == stsobj.PodName(mdb.ObjectMeta, podIndex)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should update", func() {
		By("Updating MariaDB")
		testMariadbUpdate(mdb)
	})

	It("should resize PVCs", func() {
		By("Resizing MariaDB PVCs")
		testMariadbVolumeResize(mdb, "400Mi")
	})

	It("should reconcile with MaxScale", Label("basic"), func() {
		mxs := &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "maxscale-repl",
				Namespace: testNamespace,
			},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				Replicas: 2,
				KubernetesService: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.io/loadBalancerIPs": testCidrPrefix + ".0.214",
						},
					},
				},
				GuiKubernetesService: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.io/loadBalancerIPs": testCidrPrefix + ".0.230",
						},
					},
				},
				Connection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: ptr.To("mxs-repl-conn"),
					HealthCheck: &mariadbv1alpha1.HealthCheck{
						Interval: ptr.To(metav1.Duration{Duration: 1 * time.Second}),
					},
				},
				Auth: mariadbv1alpha1.MaxScaleAuth{
					Generate: ptr.To(true),
					AdminPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: testPwdKey.Name,
							},
							Key: testPwdSecretKey,
						},
						Generate: false,
					},
				},
				TLS: &mariadbv1alpha1.MaxScaleTLS{
					Enabled:               true,
					VerifyPeerCertificate: ptr.To(true),
					VerifyPeerHost:        ptr.To(false),
					ReplicationSSLEnabled: ptr.To(true),
				},
				Metrics: &mariadbv1alpha1.MaxScaleMetrics{
					Enabled: true,
				},
			},
		}

		By("Using MariaDB with MaxScale")
		testMaxscale(mdb, mxs)
	})
})

var _ = Describe("MariaDB replication restore from backup", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	BeforeEach(func() {
		mdb = buildTestMariaDBWithRepl(key)
		applyMariadbTestConfig(mdb)

		By("Creating MariaDB with replication")
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()

		}, testHighTimeout, testInterval).Should(BeTrue())

	})

	DescribeTable(
		"should restore database",
		func(
			backupKey types.NamespacedName,
			builderFn physicalBackupBuilder,
			bootstrapFromBuilder func(backupKey types.NamespacedName) *mariadbv1alpha1.BootstrapFrom,
			cleanupFn func(backupKey types.NamespacedName) func(),
		) {
			backup := builderFn(backupKey)
			testPhysicalBackup(backup)
			// We delete the PhysicalBackup, because the job holds the pvc
			deletePhysicalBackup(backupKey, false)
			DeferCleanup(cleanupFn(backupKey))

			By("Deleting MariaDB")
			bootstrapFrom := mdb.DeepCopy()
			deleteMariadb(key, true)

			By("Creating MariaDB from PhysicalBackup")
			bootstrapFrom.ObjectMeta = metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			}
			bootstrapFrom.Spec.BootstrapFrom = bootstrapFromBuilder(backupKey)
			Expect(k8sClient.Create(testCtx, bootstrapFrom)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				return mdb.IsReady() && mdb.IsInitialized() && mdb.HasRestoredBackup()
			}, testHighTimeout, testInterval).Should(BeTrue())
		},
		Entry(
			"from physical backup",
			types.NamespacedName{Name: "replication-s3-backup-test", Namespace: key.Namespace},
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage(key, "test-replication-restore-from-backup", ""),
				decoratePhysicalBackupWithSSEC,
			),
			func(backupKey types.NamespacedName) *mariadbv1alpha1.BootstrapFrom {
				return &mariadbv1alpha1.BootstrapFrom{
					BackupContentType:  mariadbv1alpha1.BackupContentTypePhysical,
					S3:                 getS3Storage("test-replication-restore-from-backup", "", withSSEC()),
					TargetRecoveryTime: testTargetRecoveryTime,
				}
			},
			func(backupKey types.NamespacedName) func() {
				return func() {
					// No cleanup for S3
				}
			},
		),
		Entry(
			"from volume snapshot",
			types.NamespacedName{Name: "replication-volume-snapshot-backup-test", Namespace: key.Namespace},
			buildPhysicalBackupWithVolumeSnapshotStorage(key),
			func(backupKey types.NamespacedName) *mariadbv1alpha1.BootstrapFrom {
				selector := labels.SelectorFromSet(labels.Set{metadata.PhysicalBackupNameLabel: backupKey.Name})

				snapshotList := &volumesnapshotv1.VolumeSnapshotList{}
				listOpts := []client.ListOption{client.InNamespace(backupKey.Namespace), client.MatchingLabelsSelector{Selector: selector}}

				Expect(k8sClient.List(testCtx, snapshotList, listOpts...)).To(Succeed())
				Expect(snapshotList.Items).To(HaveLen(1))

				return &mariadbv1alpha1.BootstrapFrom{
					VolumeSnapshotRef: &mariadbv1alpha1.LocalObjectReference{
						Name: snapshotList.Items[0].Name,
					},
					TargetRecoveryTime: testTargetRecoveryTime,
				}
			},
			func(backupKey types.NamespacedName) func() {
				return func() {
					By("Deleting Backup Resources")
					opts := []client.DeleteAllOfOption{
						client.MatchingLabels{
							metadata.PhysicalBackupNameLabel: backupKey.Name,
						},
						client.InNamespace(backupKey.Namespace),
					}
					Expect(k8sClient.DeleteAllOf(testCtx, &volumesnapshotv1.VolumeSnapshot{}, opts...)).To(Succeed())
				}
			},
		),
	)
})

var _ = Describe("MariaDB replication scale out", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	BeforeEach(func() {
		mdb = buildTestMariaDBWithRepl(key)
		applyMariadbTestConfig(mdb)

		By("Creating MariaDB with replication")
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()

		}, testHighTimeout, testInterval).Should(BeTrue())
	})

	DescribeTable(
		"should scale out",
		func(
			backupKey types.NamespacedName,
			builderFn physicalBackupBuilder,
			cleanupFn func(backupKey types.NamespacedName) func(),
		) {
			backup := builderFn(backupKey)
			testPhysicalBackup(backup)

			DeferCleanup(func() {
				deletePhysicalBackup(backupKey, false)
				cleanupFn(backupKey)()
			})

			By("Scale Out")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}

				mdb.Spec.Replicas = mdb.Spec.Replicas + 1
				mdb.Spec.Replication.Replica.ReplicaBootstrapFrom = &mariadbv1alpha1.ReplicaBootstrapFrom{
					PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{
						Name: backupKey.Name,
					},
				}

				return k8sClient.Update(testCtx, mdb) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				return mdb.IsReady() &&
					meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeScaledOut) &&
					mdb.Status.Replicas == int32(4)

			}, testHighTimeout, testInterval).Should(BeTrue())
		},
		Entry(
			"with physical backup",
			types.NamespacedName{Name: "replication-s3-scaleout-test", Namespace: key.Namespace},
			buildPhysicalBackupWithS3Storage(key, "test-replication-scale-out", ""),
			func(backupKey types.NamespacedName) func() {
				return func() {
					// No cleanup for s3
				}
			},
		),
		Entry(
			"from volume snapshot",
			types.NamespacedName{Name: "replication-volume-snapshot-scaleout-test", Namespace: key.Namespace},
			buildPhysicalBackupWithVolumeSnapshotStorage(key),
			func(backupKey types.NamespacedName) func() {
				return func() {
					By("Deleting Backup Resources")
					opts := []client.DeleteAllOfOption{
						client.MatchingLabels{
							metadata.PhysicalBackupNameLabel: backupKey.Name,
						},
						client.InNamespace(backupKey.Namespace),
					}
					Expect(k8sClient.DeleteAllOf(testCtx, &volumesnapshotv1.VolumeSnapshot{}, opts...)).To(Succeed())
				}
			},
		),
	)
})

var _ = Describe("MariaDB replication with password", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		passwordSecretKey = types.NamespacedName{
			Name:      "mariadb-repl-password",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	BeforeAll(func() {
		passwordSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      passwordSecretKey.Name,
				Namespace: passwordSecretKey.Namespace,
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			Data: map[string][]byte{
				"password": []byte("MariaDB11!"),
			},
		}
		Expect(k8sClient.Create(testCtx, &passwordSecret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &passwordSecret)).To(Succeed())
		})

		mdb = buildTestMariaDBWithRepl(key)
		applyMariadbTestConfig(mdb)

		probe := &mariadbv1alpha1.Probe{
			InitialDelaySeconds: 10,
			TimeoutSeconds:      1,
			PeriodSeconds:       1,
			FailureThreshold:    5,
		}
		mdb.Spec.LivenessProbe = probe
		mdb.Spec.ReadinessProbe = probe

		mdb.Spec.RootPasswordSecretKeyRef = mariadbv1alpha1.GeneratedSecretKeyRef{
			SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: passwordSecretKey.Name,
				},
				Key: "password",
			},
		}

		By("Creating MariaDB with replication")
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, false)
		})
	})

	It("should update root password", func() {
		var mdb mariadbv1alpha1.MariaDB
		By("Expecting MariaDB to be ready for 10 seconds to ensure it's stable")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &mdb)).To(Succeed())
			return mdb.IsReady()
		}, testTimeout, testInterval).MustPassRepeatedly(10).Should(BeTrue())

		By("Verifying initial password")
		executeSqlInPodByIndex(&mdb, 0, "SELECT 1")

		oldPassword := "MariaDB11!"
		newPassword := "MariaDB12!"

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(testCtx, passwordSecretKey, &secret)).To(Succeed())
			secret.Data["password"] = []byte(newPassword)
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready for 10 seconds")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &mdb)).To(Succeed())
			return mdb.IsReady()
		}, testTimeout, testInterval).MustPassRepeatedly(10).Should(BeTrue())

		By("Verifying new password")
		executeSqlInPodByIndex(&mdb, 0, "SELECT 1")

		By("Updating password Secret back to old")
		Eventually(func(g Gomega) bool {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(testCtx, passwordSecretKey, &secret)).To(Succeed())
			secret.Data["password"] = []byte(oldPassword)
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready for 10 seconds")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, &mdb)).To(Succeed())
			return mdb.IsReady()
		}, testTimeout, testInterval).MustPassRepeatedly(10).Should(BeTrue())

		By("Verifying old password")
		executeSqlInPodByIndex(&mdb, 0, "SELECT 1")
	})
})
var _ = Describe("MariaDB replication from external server with filtered tables", Ordered, func() {
	const (
		filteredDB      = "filtereddb"
		replicatedTable = "replicated_table"
		excludedTable   = "excluded_table"
		otherDB         = "otherdb"
		otherTable      = "other_table"
	)

	var (
		key = testMdbERFilteredKey
		mdb = &mariadbv1alpha1.MariaDB{}
	)

	BeforeAll(func() {
		By("Getting the external MariaDB client")
		var emdb mariadbv1alpha1.ExternalMariaDB
		Expect(k8sClient.Get(testCtx, testEMdbkey, &emdb)).To(Succeed())
		refResolver := refresolver.New(k8sClient)
		externalClient, err := sqlClient.NewClientWithMariaDB(testCtx, &emdb, refResolver)
		Expect(err).To(Succeed())
		defer externalClient.Close()

		By("Creating tables on the external server")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", filteredDB))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)",
			filteredDB, replicatedTable))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)",
			filteredDB, excludedTable))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", otherDB))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)",
			otherDB, otherTable))).To(Succeed())

		By("Creating PhysicalBackup template for filtered external replication recovery")
		backupTemplate := mariadbv1alpha1.PhysicalBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testPbTemplateERFilteredKey.Name,
				Namespace: testPbTemplateERFilteredKey.Namespace,
			},
			Spec: mariadbv1alpha1.PhysicalBackupSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbERFilteredKey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: false,
				},
				Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				Schedule: &mariadbv1alpha1.PhysicalBackupSchedule{
					Suspend: true,
				},
				Compression: mariadbv1alpha1.CompressBzip2,
				Storage: mariadbv1alpha1.PhysicalBackupStorage{
					PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
				},
				Timeout:     &metav1.Duration{Duration: 1 * time.Hour},
				PodAffinity: ptr.To(true),
				JobContainerTemplate: mariadbv1alpha1.JobContainerTemplate{
					Resources: &mariadbv1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("300m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &backupTemplate)).To(Succeed())

		mdbFiltered := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Username: &testUser,
				PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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
				max_allowed_packet=256M`),
				Replication: &mariadbv1alpha1.Replication{
					ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
						ReplicaFromExternal: &mariadbv1alpha1.ReplicaFromExternal{
							MariaDBRef: mariadbv1alpha1.MariaDBRef{
								ObjectReference: mariadbv1alpha1.ObjectReference{
									Name: testEMdbkey.Name,
								},
								Kind: mariadbv1alpha1.ExternalMariaDBKind,
							},
							ServerIdOffset: ptr.To(70),
							FilteredReplicaTables: []string{
								fmt.Sprintf("%s.%s", filteredDB, replicatedTable),
							},
						},
						Replica: mariadbv1alpha1.ReplicaReplication{
							ReplicaBootstrapFrom: &mariadbv1alpha1.ReplicaBootstrapFrom{
								PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{
									Name: testPbTemplateERFilteredKey.Name,
								},
							},
							IgnoreMaxLagSeconds:             ptr.To(true),
							IgnoreReplicationLivenessProbes: ptr.To(true),
						},
					},
					Enabled: true,
				},
				Replicas: 2,
				Storage: mariadbv1alpha1.Storage{
					Size:                ptr.To(resource.MustParse("300Mi")),
					StorageClassName:    "standard-resize",
					ResizeInUseVolumes:  ptr.To(true),
					WaitForVolumeResize: ptr.To(true),
				},
				TLS: &mariadbv1alpha1.TLS{
					Enabled:  true,
					Required: ptr.To(true),
				},
				Service: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.188",
						},
					},
				},
				Connection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: func() *string {
						s := "mdb-repl-ext-filtered-conn"
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
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.189",
						},
					},
				},
				PrimaryConnection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: func() *string {
						s := "mdb-repl-ext-filtered-conn-primary"
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
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.194",
						},
					},
				},
				SecondaryConnection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: func() *string {
						s := "mdb-repl-ext-filtered-conn-secondary"
						return &s
					}(),
					SecretTemplate: &mariadbv1alpha1.SecretTemplate{
						Key: &testConnSecretKey,
					},
				},
				UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
					Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
				},
			},
		}
		applyMariadbTestConfig(mdbFiltered)
		By("Creating MariaDB with filtered external replication")
		Expect(k8sClient.Create(testCtx, mdbFiltered)).To(Succeed())

		DeferCleanup(func() {
			var pbTemplate mariadbv1alpha1.PhysicalBackup
			if err := k8sClient.Get(testCtx, testPbTemplateERFilteredKey, &pbTemplate); err == nil {
				Expect(k8sClient.Delete(testCtx, &pbTemplate)).To(Succeed())
			}
			var pbRecoveryPvc corev1.PersistentVolumeClaim
			if err := k8sClient.Get(testCtx, testMdbPbRecoveryERFilteredKey, &pbRecoveryPvc); err == nil {
				Expect(k8sClient.Delete(testCtx, &pbRecoveryPvc)).To(Succeed())
			}
			deleteMariadb(key, false)
		})
	})

	It("should reconcile", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())
	})

	It("should only have the filtered table after initial restore", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		refResolver := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			By("Expecting to get SQL client for Pod " + strconv.Itoa(i))
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Not able get SQL for POD: %v \n", err)
			}
			Expect(err).To(Succeed())
			defer podClient.Close()

			By(fmt.Sprintf("Expecting Pod %d to have the replicated table", i))
			exists, err := podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				filteredDB, replicatedTable,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			By(fmt.Sprintf("Expecting Pod %d to NOT have the excluded table from the same database", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				filteredDB, excludedTable,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			By(fmt.Sprintf("Expecting Pod %d to NOT have the other database table", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				otherDB, otherTable,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())
		}
	})

	It("should replicate only changes to the filtered table", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		By("Getting the external MariaDB client")
		var emdb mariadbv1alpha1.ExternalMariaDB
		Expect(k8sClient.Get(testCtx, testEMdbkey, &emdb)).To(Succeed())
		refResolver := refresolver.New(k8sClient)
		externalClient, err := sqlClient.NewClientWithMariaDB(testCtx, &emdb, refResolver)
		Expect(err).To(Succeed())
		defer externalClient.Close()

		By("Inserting a row into the replicated table on the external server")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"INSERT IGNORE INTO `%s`.`%s` VALUES (42)", filteredDB, replicatedTable,
		))).To(Succeed())

		By("Expecting the inserted row to appear on all replicas")
		refResolver2 := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, pErr := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver2, i)
			Expect(pErr).To(Succeed())
			defer podClient.Close()

			Eventually(func() bool {
				exists, eErr := podClient.Exists(testCtx, fmt.Sprintf(
					"SELECT 1 FROM `%s`.`%s` WHERE id = 42", filteredDB, replicatedTable,
				))
				return eErr == nil && exists
			}, testTimeout, testInterval).Should(BeTrue(),
				fmt.Sprintf("Pod %d should have the replicated row", i))
		}
	})

	It("should have GTID strict mode disabled", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		refResolver := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			Expect(err).To(Succeed())
			defer podClient.Close()

			By(fmt.Sprintf("Expecting GTID strict mode to be disabled on Pod %d", i))
			val, err := podClient.SystemVariable(testCtx, "gtid_strict_mode")
			Expect(err).To(Succeed())
			fmt.Fprintf(GinkgoWriter, "gtid_strict_mode: %v \n", val)
			Expect(val).To(Equal("0"))
		}
	})
})

var _ = Describe("MariaDB replication from external server with filtered tables from multiple schemas", Ordered, func() {
	const (
		schema1             = "multischema1db"
		schema2             = "multischema2db"
		otherSchema         = "otherschemadb"
		replicatedInSchema1 = "replicated_in_schema1"
		replicatedInSchema2 = "replicated_in_schema2"
		excludedInSchema1   = "excluded_in_schema1"
		excludedInSchema2   = "excluded_in_schema2"
		otherSchemaTable    = "other_table"
		viewOnExcluded1     = "view_on_excluded_schema1"
		viewOnExcluded2     = "view on excluded schema2"
	)

	var (
		key = testMdbERMultiSchemaKey
		mdb = &mariadbv1alpha1.MariaDB{}
	)

	BeforeAll(func() {
		By("Getting the external MariaDB client")
		var emdb mariadbv1alpha1.ExternalMariaDB
		Expect(k8sClient.Get(testCtx, testEMdbkey, &emdb)).To(Succeed())
		refResolver := refresolver.New(k8sClient)
		externalClient, err := sqlClient.NewClientWithMariaDB(testCtx, &emdb, refResolver)
		Expect(err).To(Succeed())
		defer externalClient.Close()

		By("Creating tables on the external server across two schemas")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", schema1))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)", schema1, replicatedInSchema1,
		))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)", schema1, excludedInSchema1,
		))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", schema2))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)", schema2, replicatedInSchema2,
		))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)", schema2, excludedInSchema2,
		))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", otherSchema))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS `%s`.`%s` (id INT PRIMARY KEY)", otherSchema, otherSchemaTable,
		))).To(Succeed())

		By("Creating views that reference excluded tables (must be ignored by the dump)")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE OR REPLACE VIEW `%s`.`%s` AS SELECT * FROM `%s`.`%s`",
			schema1, viewOnExcluded1, schema1, excludedInSchema1,
		))).To(Succeed())
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"CREATE OR REPLACE VIEW `%s`.`%s` AS SELECT * FROM `%s`.`%s`",
			schema2, viewOnExcluded2, schema2, excludedInSchema2,
		))).To(Succeed())

		By("Creating PhysicalBackup template for multi-schema filtered external replication recovery")
		backupTemplate := mariadbv1alpha1.PhysicalBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testPbTemplateERMultiSchemaKey.Name,
				Namespace: testPbTemplateERMultiSchemaKey.Namespace,
			},
			Spec: mariadbv1alpha1.PhysicalBackupSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: testMdbERMultiSchemaKey.Name,
					},
					Kind:      mariadbv1alpha1.ExternalMariaDBKind,
					WaitForIt: false,
				},
				Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				Schedule: &mariadbv1alpha1.PhysicalBackupSchedule{
					Suspend: true,
				},
				Compression: mariadbv1alpha1.CompressBzip2,
				Storage: mariadbv1alpha1.PhysicalBackupStorage{
					PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
				},
				Timeout:     &metav1.Duration{Duration: 1 * time.Hour},
				PodAffinity: ptr.To(true),
				JobContainerTemplate: mariadbv1alpha1.JobContainerTemplate{
					Resources: &mariadbv1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("300m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, &backupTemplate)).To(Succeed())

		mdbMultiSchema := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Username: &testUser,
				PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
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
				max_allowed_packet=256M`),
				Replication: &mariadbv1alpha1.Replication{
					ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
						ReplicaFromExternal: &mariadbv1alpha1.ReplicaFromExternal{
							MariaDBRef: mariadbv1alpha1.MariaDBRef{
								ObjectReference: mariadbv1alpha1.ObjectReference{
									Name: testEMdbkey.Name,
								},
								Kind: mariadbv1alpha1.ExternalMariaDBKind,
							},
							ServerIdOffset: ptr.To(80),
							FilteredReplicaTables: []string{
								fmt.Sprintf("%s.%s", schema1, replicatedInSchema1),
								fmt.Sprintf("%s.%s", schema2, replicatedInSchema2),
							},
						},
						Replica: mariadbv1alpha1.ReplicaReplication{
							ReplicaBootstrapFrom: &mariadbv1alpha1.ReplicaBootstrapFrom{
								PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{
									Name: testPbTemplateERMultiSchemaKey.Name,
								},
							},
							IgnoreMaxLagSeconds:             ptr.To(true),
							IgnoreReplicationLivenessProbes: ptr.To(true),
						},
					},
					Enabled: true,
				},
				Replicas: 2,
				Storage: mariadbv1alpha1.Storage{
					Size:                ptr.To(resource.MustParse("300Mi")),
					StorageClassName:    "standard-resize",
					ResizeInUseVolumes:  ptr.To(true),
					WaitForVolumeResize: ptr.To(true),
				},
				TLS: &mariadbv1alpha1.TLS{
					Enabled:  true,
					Required: ptr.To(true),
				},
				Service: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.195",
						},
					},
				},
				Connection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: func() *string {
						s := "mdb-repl-ext-multi-schema-conn"
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
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.196",
						},
					},
				},
				PrimaryConnection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: func() *string {
						s := "mdb-repl-ext-multi-schema-conn-primary"
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
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.197",
						},
					},
				},
				SecondaryConnection: &mariadbv1alpha1.ConnectionTemplate{
					SecretName: func() *string {
						s := "mdb-repl-ext-multi-schema-conn-secondary"
						return &s
					}(),
					SecretTemplate: &mariadbv1alpha1.SecretTemplate{
						Key: &testConnSecretKey,
					},
				},
				UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
					Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
				},
			},
		}
		applyMariadbTestConfig(mdbMultiSchema)
		By("Creating MariaDB with multi-schema filtered external replication")
		Expect(k8sClient.Create(testCtx, mdbMultiSchema)).To(Succeed())

		DeferCleanup(func() {
			var pbTemplate mariadbv1alpha1.PhysicalBackup
			if err := k8sClient.Get(testCtx, testPbTemplateERMultiSchemaKey, &pbTemplate); err == nil {
				Expect(k8sClient.Delete(testCtx, &pbTemplate)).To(Succeed())
			}
			var pbRecoveryPvc corev1.PersistentVolumeClaim
			if err := k8sClient.Get(testCtx, testMdbPbRecoveryERMultiSchemaKey, &pbRecoveryPvc); err == nil {
				Expect(k8sClient.Delete(testCtx, &pbRecoveryPvc)).To(Succeed())
			}
			deleteMariadb(key, false)
		})
	})

	It("should reconcile", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())
	})

	It("should have only the filtered tables from each schema after initial restore", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		refResolver := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			By("Expecting to get SQL client for Pod " + strconv.Itoa(i))
			Expect(err).To(Succeed())
			defer podClient.Close()

			By(fmt.Sprintf("Expecting Pod %d to have the replicated table from schema1", i))
			exists, err := podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				schema1, replicatedInSchema1,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			By(fmt.Sprintf("Expecting Pod %d to have the replicated table from schema2", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				schema2, replicatedInSchema2,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			By(fmt.Sprintf("Expecting Pod %d to NOT have the excluded table from schema1", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				schema1, excludedInSchema1,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			By(fmt.Sprintf("Expecting Pod %d to NOT have the excluded table from schema2", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				schema2, excludedInSchema2,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			By(fmt.Sprintf("Expecting Pod %d to NOT have any table from the excluded schema", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
				otherSchema, otherSchemaTable,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			By(fmt.Sprintf("Expecting Pod %d to NOT have the view referencing the excluded schema1 table", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.views WHERE table_schema='%s' AND table_name='%s'",
				schema1, viewOnExcluded1,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			By(fmt.Sprintf("Expecting Pod %d to NOT have the view referencing the excluded schema2 table", i))
			exists, err = podClient.Exists(testCtx, fmt.Sprintf(
				"SELECT 1 FROM information_schema.views WHERE table_schema='%s' AND table_name='%s'",
				schema2, viewOnExcluded2,
			))
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())
		}
	})

	It("should replicate changes to the filtered table in each schema", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		By("Getting the external MariaDB client")
		var emdb mariadbv1alpha1.ExternalMariaDB
		Expect(k8sClient.Get(testCtx, testEMdbkey, &emdb)).To(Succeed())
		refResolver := refresolver.New(k8sClient)
		externalClient, err := sqlClient.NewClientWithMariaDB(testCtx, &emdb, refResolver)
		Expect(err).To(Succeed())
		defer externalClient.Close()

		By("Inserting a row into the replicated table in schema1 on the external server")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"INSERT IGNORE INTO `%s`.`%s` VALUES (1)", schema1, replicatedInSchema1,
		))).To(Succeed())

		By("Inserting a row into the replicated table in schema2 on the external server")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"INSERT IGNORE INTO `%s`.`%s` VALUES (2)", schema2, replicatedInSchema2,
		))).To(Succeed())

		By("Expecting inserted rows from both schemas to appear on all replicas")
		refResolver2 := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, pErr := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver2, i)
			Expect(pErr).To(Succeed())
			defer podClient.Close()

			Eventually(func() bool {
				exists, eErr := podClient.Exists(testCtx, fmt.Sprintf(
					"SELECT 1 FROM `%s`.`%s` WHERE id = 1", schema1, replicatedInSchema1,
				))
				return eErr == nil && exists
			}, testTimeout, testInterval).Should(BeTrue(),
				fmt.Sprintf("Pod %d should have the schema1 replicated row", i))

			Eventually(func() bool {
				exists, eErr := podClient.Exists(testCtx, fmt.Sprintf(
					"SELECT 1 FROM `%s`.`%s` WHERE id = 2", schema2, replicatedInSchema2,
				))
				return eErr == nil && exists
			}, testTimeout, testInterval).Should(BeTrue(),
				fmt.Sprintf("Pod %d should have the schema2 replicated row", i))
		}

		By("Expecting changes to the excluded table in schema1 NOT to be replicated")
		Expect(externalClient.Exec(testCtx, fmt.Sprintf(
			"INSERT IGNORE INTO `%s`.`%s` VALUES (99)", schema1, excludedInSchema1,
		))).To(Succeed())

		refResolver3 := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, pErr := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver3, i)
			Expect(pErr).To(Succeed())
			defer podClient.Close()

			Consistently(func() bool {
				exists, eErr := podClient.Exists(testCtx, fmt.Sprintf(
					"SELECT 1 FROM information_schema.tables WHERE table_schema='%s' AND table_name='%s'",
					schema1, excludedInSchema1,
				))
				return eErr == nil && !exists
			}, 10*time.Second, testInterval).Should(BeTrue(),
				fmt.Sprintf("Pod %d should never receive the excluded schema1 table", i))
		}
	})

	It("should have GTID strict mode disabled", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		refResolver := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			Expect(err).To(Succeed())
			defer podClient.Close()

			By(fmt.Sprintf("Expecting GTID strict mode to be disabled on Pod %d", i))
			val, err := podClient.SystemVariable(testCtx, "gtid_strict_mode")
			Expect(err).To(Succeed())
			fmt.Fprintf(GinkgoWriter, "gtid_strict_mode: %v \n", val)
			Expect(val).To(Equal("0"))
		}
	})
})
