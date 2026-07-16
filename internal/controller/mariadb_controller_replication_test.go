package controller

import (
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
			deleteMariadb(key, true)
		})
	})

	It("should reconcile", Label("basic"), func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Service")
		var svc corev1.Service
		Expect(k8sClient.Get(testCtx, key, &svc)).To(Succeed())

		By("Expecting to create a primary Service")
		Expect(k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc)).To(Succeed())
		Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(mdb.ObjectMeta, 0)))

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

	It("should repair live SQL role drift", func() {
		By("Expecting MariaDB primary to be set")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.Status.CurrentPrimaryPodIndex != nil
		}, testTimeout, testInterval).Should(BeTrue())

		primaryPodIndex := *mdb.Status.CurrentPrimaryPodIndex
		replicaPodIndex := -1
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			if i != primaryPodIndex {
				replicaPodIndex = i
				break
			}
		}
		Expect(replicaPodIndex).ToNot(Equal(-1))

		By("Expecting live SQL roles to be healthy")
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, primaryPodIndex)
			return err == nil && !readOnly
		}, testTimeout, testInterval).Should(BeTrue())
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, replicaPodIndex)
			return err == nil && readOnly
		}, testTimeout, testInterval).Should(BeTrue())

		By("Corrupting live SQL role state")
		executeSqlInPodByIndex(mdb, primaryPodIndex, "SET GLOBAL read_only=1;")
		executeSqlInPodByIndex(mdb, replicaPodIndex, "SET GLOBAL read_only=0;")
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, primaryPodIndex)
			return err == nil && readOnly
		}, testTimeout, testInterval).Should(BeTrue())
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, replicaPodIndex)
			return err == nil && !readOnly
		}, testTimeout, testInterval).Should(BeTrue())

		By("Triggering MariaDB reconciliation")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			if mdb.Annotations == nil {
				mdb.Annotations = make(map[string]string)
			}
			mdb.Annotations["k8s.mariadb.com/test-reconcile-at"] = time.Now().Format(time.RFC3339Nano)
			g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting live SQL role drift to be repaired")
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, primaryPodIndex)
			return err == nil && !readOnly
		}, testHighTimeout, testInterval).Should(BeTrue())
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, replicaPodIndex)
			return err == nil && readOnly
		}, testHighTimeout, testInterval).Should(BeTrue())
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
			return svc.Spec.Selector["statefulset.kubernetes.io/pod-name"] == statefulset.PodName(mdb.ObjectMeta, podIndex)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should preserve GTID history during switchover", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() && mdb.Status.CurrentPrimaryPodIndex != nil
		}, testHighTimeout, testInterval).Should(BeTrue())

		currentPrimaryIndex := *mdb.Status.CurrentPrimaryPodIndex
		clientSet := sql.NewClientSet(mdb, refresolver.New(k8sClient))
		currentPrimaryClient, err := clientSet.ClientForIndex(testCtx, currentPrimaryIndex)
		Expect(err).ToNot(HaveOccurred())
		currentPrimaryGTID, err := currentPrimaryClient.GtidBinlogPos(testCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(currentPrimaryClient.Close()).To(Succeed())
		currentPrimaryParsedGTID, err := replication.ParseGtid(currentPrimaryGTID)
		Expect(err).ToNot(HaveOccurred())

		newPrimaryIndex := 0
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			if i != currentPrimaryIndex {
				newPrimaryIndex = i
				break
			}
		}

		By("Switching primary")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			mdb.Spec.Replication.Primary.PodIndex = &newPrimaryIndex
			g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually change primary")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() && mdb.Status.CurrentPrimaryPodIndex != nil &&
				*mdb.Status.CurrentPrimaryPodIndex == newPrimaryIndex
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting promoted primary to preserve replicated GTID history")
		newPrimaryClient, err := clientSet.ClientForIndex(testCtx, newPrimaryIndex)
		Expect(err).ToNot(HaveOccurred())
		newPrimaryGTID, err := newPrimaryClient.GtidCurrentPos(testCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(newPrimaryClient.Close()).To(Succeed())
		newPrimaryParsedGTID, err := replication.ParseFurthestGtidWithDomainId(
			newPrimaryGTID,
			currentPrimaryParsedGTID.DomainID,
			testLogger,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(newPrimaryParsedGTID.SequenceID).To(BeNumerically(">=", currentPrimaryParsedGTID.SequenceID))
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
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.214",
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
			deleteMariadb(key, true)
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
			deletePhysicalBackup(backupKey)
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
			deleteMariadb(key, true)
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
				deletePhysicalBackup(backupKey)
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
