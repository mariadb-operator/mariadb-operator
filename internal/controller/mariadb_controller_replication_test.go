package controller

import (
	"fmt"
	"strconv"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
		mdb = &mariadbv1alpha1.MariaDB{
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
				max_allowed_packet=256M`,
				),
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
						SyncBinlog: ptr.To(1),
					},
					Enabled: true,
				},
				Replicas: 3,
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
				UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
					Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
				},
			},
		}
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

	It("should fail and switch over primary", func() {
		Skip("TODO: re-evaluate this test when productionizing replication. See https://github.com/mariadb-operator/mariadb-operator/issues/738")

		By("Expecting MariaDB primary to be set")
		Eventually(func() bool {
			return mdb.Status.CurrentPrimary != nil
		}, testTimeout, testInterval).Should(BeTrue())

		currentPrimary := *mdb.Status.CurrentPrimary
		By("Tearing down primary Pod consistently")
		Consistently(func() bool {
			primaryPodKey := types.NamespacedName{
				Name:      currentPrimary,
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
			mdb.Replication().Primary.PodIndex = &podIndex
			g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

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

	It("should update", func() {
		By("Updating MariaDB")
		testMariadbUpdate(mdb)
	})

	It("should resize PVCs", func() {
		By("Resizing MariaDB PVCs")
		testMariadbVolumeResize(mdb, "400Mi")
	})

	It("should reconcile with MaxScale", Label("basic"), func() {
		Skip("TODO: re-evaluate this test when productionizing replication. See https://github.com/mariadb-operator/mariadb-operator/issues/738")

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

var _ = Describe("MariaDB replication from external server", Ordered, func() {

	var (
		key = testMdbERkey
		mdb = &mariadbv1alpha1.MariaDB{}
	)

	It("should reconcile", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			fmt.Fprintf(GinkgoWriter, "Trying to get %v \n", testMdbERkey.Name)
			if err := k8sClient.Get(testCtx, testMdbERkey, mdb); err != nil {
				fmt.Fprintf(GinkgoWriter, "Error %v \n", err)
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
		var endpoints discoveryv1.EndpointSlice

		By("Expecting to create secondary Endpoints: " + strconv.Itoa(int(mdb.Spec.Replicas)))
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == int(mdb.Spec.Replicas)

		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a PodDisruptionBudget")
		var pdb policyv1.PodDisruptionBudget
		Expect(k8sClient.Get(testCtx, key, &pdb)).To(Succeed())
	})

	It("should restart replication if stopped", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to get SqlClient from Pod 2")
		refResolver := refresolver.New(k8sClient)
		var client *sqlClient.Client
		var err error
		podIndex := 2
		client, err = sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, podIndex)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Expecting to stop replication on Pod 2")
		Expect(client.Exec(testCtx, "STOP SLAVE")).To(Succeed())

		By("Expecting replication to be ready eventually on Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			isReplicaHealthy, _ := client.IsReplicationHealthy(testCtx)
			return isReplicaHealthy
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting replication status to get back to slave Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return apierrors.IsNotFound(err)
			}
			return mdb.Status.ReplicationStatus[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationStateSlave
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting Pod " + strconv.Itoa(podIndex) + " to present on the secondary endpoints")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())

			podKey := types.NamespacedName{
				Name:      statefulset.PodName(mdb.ObjectMeta, podIndex),
				Namespace: testNamespace,
			}
			var pod corev1.Pod
			Expect(k8sClient.Get(testCtx, podKey, &pod)).To(Succeed())

			for _, address := range endpoints.Endpoints {
				if address.Addresses[0] == pod.Status.PodIP && *address.Conditions.Ready {
					return true
				}
			}
			return false
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should rebuild Pod and PVC in case of permanent replication issue", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to get SqlClient from Pod 2")
		refResolver := refresolver.New(k8sClient)
		var client *sqlClient.Client
		var err error
		podIndex := 2
		client, err = sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, podIndex)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Suspend MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Suspend = true

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually be suspended")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			condition := meta.FindStatusCondition(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReady)
			if condition == nil {
				return false
			}
			return condition.Status == metav1.ConditionFalse && condition.Reason == mariadbv1alpha1.ConditionReasonSuspended
		})

		By("Expecting to stop replication on Pod 2")
		Expect(client.Exec(testCtx, "STOP SLAVE")).To(Succeed(), client.Exec(testCtx, "SET GLOBAL gtid_slave_pos = '0-9999-9999'"))

		By("Expecting to set Invalid GTID position on Pod 2")
		Expect(client.Exec(testCtx, "SET GLOBAL gtid_slave_pos = '0-9999-9999'")).To(Succeed())

		By("Expecting to start replication on Pod 2")
		Expect(client.Exec(testCtx, "START SLAVE")).To(Succeed())

		By("Expecting replication error 1236 on Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			rStatus, err := client.GetReplicationStatus(testCtx)
			if err != nil {
				return false
			}

			return rStatus.LastIOErrno.Int32 == 1236

		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Resume MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Suspend = false

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting no replication error 1236 on Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			rStatus, err := client.GetReplicationStatus(testCtx)
			if err != nil {
				return false
			}

			return rStatus.LastIOErrno.Int32 != 1236

		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting replication status to get back to slave Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return apierrors.IsNotFound(err)
			}
			return mdb.Status.ReplicationStatus[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationStateSlave
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting Pod " + strconv.Itoa(podIndex) + " to present on the secondary endpoints")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())

			podKey := types.NamespacedName{
				Name:      statefulset.PodName(mdb.ObjectMeta, podIndex),
				Namespace: testNamespace,
			}
			var pod corev1.Pod
			Expect(k8sClient.Get(testCtx, podKey, &pod)).To(Succeed())

			for _, address := range endpoints.Endpoints {
				if address.Addresses[0] == pod.Status.PodIP && *address.Conditions.Ready {
					return true
				}
			}
			return false
		}, testTimeout, testInterval).Should(BeTrue())

	})

	It("should reuse backup if backup still valid", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		// Get current backup age
		By("Expecting to get current external MariadDB object")
		refResolver := refresolver.New(k8sClient)
		emdb, err := refResolver.ExternalMariaDB(testCtx, &mdb.Replication().ReplicaFromExternal.MariaDBRef, testNamespace)
		Expect(err).To(Succeed())

		key := types.NamespacedName{
			Name:      emdb.Name,
			Namespace: emdb.Namespace,
		}
		var existingBackup mariadbv1alpha1.Backup
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, key, &existingBackup)
		Expect(err).To(Succeed())
		firstBackupCreationTimestamp := existingBackup.CreationTimestamp.Time

		testDeletePod(mdb, 2, true)

		// Get current backup age
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, key, &existingBackup)
		Expect(err).To(Succeed())
		secondBackupCreationTimestamp := existingBackup.CreationTimestamp.Time

		// Last age should be older than first
		By("Expecting to have same CreationTimestamp on backup Object before and after the Pod recreation")
		Expect(firstBackupCreationTimestamp).To(Equal(secondBackupCreationTimestamp))

	})

	It("should invalidate backup if older than the master binlog retention period", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		// Get current backup age
		By("Expecting to get current external MariadDB object")
		refResolver := refresolver.New(k8sClient)
		emdb, err := refResolver.ExternalMariaDB(testCtx, &mdb.Replication().ReplicaFromExternal.MariaDBRef, testNamespace)
		Expect(err).To(Succeed())

		key := types.NamespacedName{
			Name:      emdb.Name,
			Namespace: emdb.Namespace,
		}
		var existingBackup mariadbv1alpha1.Backup
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, key, &existingBackup)
		Expect(err).To(Succeed())
		firstBackupCreationTimestamp := existingBackup.CreationTimestamp.Time

		podIndex := 2

		// Change binlog_expire_logs_seconds to 10 on the master server
		By("Expecting to get SqlClient from the external MariaDB")
		client, err := sqlClient.NewClientWithMariaDB(testCtx, emdb, refResolver)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Expecting to set binlog_expire_logs_seconds to 10 on the master server")
		Expect(client.SetSystemVariable(testCtx, "binlog_expire_logs_seconds", "60")).To(Succeed())

		testDeletePod(mdb, podIndex, true)

		// Get current backup age
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, key, &existingBackup)
		Expect(err).To(Succeed())
		secondBackupCreationTimestamp := existingBackup.CreationTimestamp.Time

		// Last age should be older than first
		By("Expecting to have same CreationTimestamp on backup Object before and after the Pod recreation")
		Expect(firstBackupCreationTimestamp).ShouldNot(Equal(secondBackupCreationTimestamp))

		// Revert binlog_expire_logs_seconds to 30 days on the master server
		By("Expecting to set expire_logs_days to 30 on the master server")
		Expect(client.SetSystemVariable(testCtx, "expire_logs_days", "30")).To(Succeed())
	})

	It("scale out replicas", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Increasing MariaDB replicas to 4")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Replicas = 4

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting to create secondary Endpoints: 4")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == 4
		}, testTimeout, testInterval).Should(BeTrue())

	})

	It("scale in replicas", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Decreasing MariaDB replicas to 3")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Replicas = 3

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting secondary Endpoints: 3")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == 3
		}, testTimeout, testInterval).Should(BeTrue())

	})

	It("use the server_id offset", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		offset := mdb.Replication().ReplicaFromExternal.ServerIdOffset
		replicas := int(mdb.Spec.Replicas)
		refResolver := refresolver.New(k8sClient)
		for i := 0; i < replicas; i++ {

			client, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			By("Expecting to get SqlClient from Pod " + strconv.Itoa(i))
			Expect(err).To(Succeed())

			server_id, err := client.SystemVariable(testCtx, "server_id")
			By("Expecting to get server_id from Pod " + strconv.Itoa(i))
			Expect(err).To(Succeed())

			server_id_int, _ := strconv.Atoi(server_id)

			By("Expecting server_id to be equal to podIndex + ServerIdOffset on Pod " + strconv.Itoa(i))
			Expect(server_id_int).To(Equal(i + *offset))

		}
	})

	It("should update", func() {
		By("Updating MariaDB")
		testMariadbUpdate(mdb)
	})

	It("should resize PVCs", func() {
		By("Resizing MariaDB PVCs")
		testMariadbVolumeResize(mdb, "400Mi")
	})

})
