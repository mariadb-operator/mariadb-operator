package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	replicationctrl "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var (
	primaryKey = types.NamespacedName{
		Name:      "mariadb-eu-south",
		Namespace: testNamespace,
	}
	primaryMdb              *mariadbv1alpha1.MariaDB
	primaryExternalMdb      *mariadbv1alpha1.ExternalMariaDB
	primaryGtidDomainId     = ptr.To(0)
	primaryServerStartIndex = ptr.To(10)
	primaryBackup           *mariadbv1alpha1.PhysicalBackup
	primaryMaxScaleKey      = types.NamespacedName{
		Name:      "maxscale-eu-south",
		Namespace: testNamespace,
	}
	primaryMaxScale *mariadbv1alpha1.MaxScale

	replicaKey = types.NamespacedName{
		Name:      "mariadb-eu-central",
		Namespace: testNamespace,
	}
	replicaMdb              *mariadbv1alpha1.MariaDB
	replicaExternalMdb      *mariadbv1alpha1.ExternalMariaDB
	replicaGtidDomainId     = ptr.To(1)
	replicaServerStartIndex = ptr.To(20)
	replicaMaxScaleKey      = types.NamespacedName{
		Name:      "maxscale-eu-central",
		Namespace: testNamespace,
	}
	replicaMaxScale *mariadbv1alpha1.MaxScale

	multiCluster = mariadbv1alpha1.MultiCluster{
		Enabled: true,
		MultiClusterSpec: mariadbv1alpha1.MultiClusterSpec{
			Primary: primaryKey.Name,
			Replicas: []string{
				replicaKey.Name,
			},
			Members: []mariadbv1alpha1.MultiClusterMember{
				{
					Name: primaryKey.Name,
					ExternalMariaDBRef: mariadbv1alpha1.ObjectReference{
						Name:      primaryKey.Name,
						Namespace: testNamespace,
					},
				},
				{
					Name: replicaKey.Name,
					ExternalMariaDBRef: mariadbv1alpha1.ObjectReference{
						Name:      replicaKey.Name,
						Namespace: testNamespace,
					},
				},
			},
		},
	}
)

var _ = Describe("MariaDB multi-cluster with replication", Ordered, Focus, func() {
	BeforeAll(func() {
		primaryBackup = buildPhysicalBackupWithS3Storage(
			primaryKey,
			"test-multi-cluster",
			primaryKey.Name,
		)(primaryKey)

		primaryMdb = applyDecoratorChain(
			multiClusterMariaDBBuilder(
				prefixedIPAddr(".1.10"),
				multiCluster,
			),
			multiClusterReplicationDecorator(
				primaryGtidDomainId,
				primaryServerStartIndex,
			),
		)(primaryKey)
		replicaMdb = applyDecoratorChain(
			multiClusterMariaDBBuilder(
				prefixedIPAddr(".1.15"),
				multiCluster,
			),
			multiClusterReplicationDecorator(
				replicaGtidDomainId,
				replicaServerStartIndex,
			),
			mariadbBootstrapFromDecorator(
				&mariadbv1alpha1.BootstrapFrom{
					BackupContentType: mariadbv1alpha1.BackupContentTypePhysical,
					S3:                primaryBackup.Spec.Storage.S3,
				},
			),
		)(replicaKey)

		primaryExternalMdb = buildMultiClusterExternalMariadb(
			primaryKey,
			statefulset.ServiceFQDN(metav1.ObjectMeta{
				Name:      primaryMdb.PrimaryServiceKey().Name,
				Namespace: primaryMdb.Namespace,
			}),
		)
		replicaExternalMdb = buildMultiClusterExternalMariadb(
			replicaKey,
			statefulset.ServiceFQDN(metav1.ObjectMeta{
				Name:      replicaMdb.PrimaryServiceKey().Name,
				Namespace: replicaMdb.Namespace,
			}),
		)
	})

	AfterAll(func() {
		deletePhysicalBackup(primaryKey, true)
		deleteMariadb(primaryKey, true)
		deleteExternalMariadb(primaryKey)

		deleteMariadb(replicaKey, true)
		deleteExternalMariadb(replicaKey)
	})

	It("should reconcile primary cluster", func() {
		By("Creating primary MariaDB")
		Expect(k8sClient.Create(testCtx, primaryMdb)).To(Succeed())

		By("Expecting primary MariaDB to be ready eventually")
		expectMariadbFn(testCtx, k8sClient, primaryKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsReady()
		})

		By("Creating primary ExternalMariaDB")
		Expect(k8sClient.Create(testCtx, primaryExternalMdb)).To(Succeed())

		By("Expecting primary ExternalMariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, primaryKey, primaryExternalMdb); err != nil {
				return false
			}
			return primaryExternalMdb.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should create a physical backup of primary cluster", func() {
		By("Creating PhysicalBackup")
		Expect(k8sClient.Create(testCtx, primaryBackup)).To(Succeed())

		By("Expecting PhysicalBackup to complete eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, primaryKey, primaryBackup); err != nil {
				return false
			}
			return primaryBackup.IsComplete()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should create replica cluster from physical backup", func() {
		By("Creating replica MariaDB")
		Expect(k8sClient.Create(testCtx, replicaMdb)).To(Succeed())

		By("Expecting MariaDB to be ready eventually")
		expectMariadbFn(testCtx, k8sClient, replicaKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsReady()
		})

		By("Creating replica ExternalMariaDB")
		Expect(k8sClient.Create(testCtx, replicaExternalMdb)).To(Succeed())

		By("Expecting ExternalMariaDB to eventually be ready")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, replicaKey, replicaExternalMdb); err != nil {
				return false
			}
			return replicaExternalMdb.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should allow to perform writes in the primary", func() {
		Expect(k8sClient.Get(testCtx, primaryKey, primaryMdb)).To(Succeed())
		Expect(primaryMdb.Status.CurrentPrimaryPodIndex).NotTo(BeNil())
		podIndex := *primaryMdb.Status.CurrentPrimaryPodIndex

		// in addition to verify writes, this increases gtid_current_pos, to be validated in the next step.
		By("Writing in primary MariaDB")
		query := `CREATE DATABASE IF NOT EXISTS test;`
		executeSqlInPodByIndex(primaryMdb, podIndex, query)
		query = `CREATE TABLE IF NOT EXISTS test.test (id INT PRIMARY KEY AUTO_INCREMENT, test VARCHAR(100));`
		executeSqlInPodByIndex(primaryMdb, podIndex, query)
		query = `INSERT INTO test.test (test) VALUES ('test');`
		executeSqlInPodByIndex(primaryMdb, podIndex, query)
	})

	It("should have valid replication status", testReplicationStatus)

	It("should perform switchover in primary cluster", func() {
		Expect(k8sClient.Get(testCtx, primaryKey, primaryMdb)).To(Succeed())
		Expect(primaryMdb.Status.CurrentPrimaryPodIndex).NotTo(BeNil())
		currentPrimaryPodIndex := primaryMdb.Status.CurrentPrimaryPodIndex
		var newPrimaryPodIndex *int
		for i := 0; i < int(primaryMdb.Spec.Replicas); i++ {
			if i != *currentPrimaryPodIndex {
				newPrimaryPodIndex = &i
				break
			}
		}

		By("Triggering switchover in primary MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, primaryKey, primaryMdb); err != nil {
				return false
			}
			primaryMdb.Spec.Replication.Primary.PodIndex = newPrimaryPodIndex
			return k8sClient.Update(testCtx, primaryMdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting primary MariaDB to be switched over eventually")
		expectMariadbFn(testCtx, k8sClient, primaryKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			if mdb.Status.CurrentPrimaryPodIndex == nil {
				return false
			}
			return mdb.IsReady() && *mdb.Status.CurrentPrimaryPodIndex != *currentPrimaryPodIndex
		})

		By("Expecting replica MariaDB to be ready")
		expectMariadbFn(testCtx, k8sClient, replicaKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsReady()
		})
	})

	It("should perform switchover in replica cluster", func() {
		Expect(k8sClient.Get(testCtx, replicaKey, replicaMdb)).To(Succeed())
		Expect(replicaMdb.Status.CurrentPrimaryPodIndex).NotTo(BeNil())
		currentPrimaryPodIndex := replicaMdb.Status.CurrentPrimaryPodIndex
		var newPrimaryPodIndex *int
		for i := 0; i < int(replicaMdb.Spec.Replicas); i++ {
			if i != *currentPrimaryPodIndex {
				newPrimaryPodIndex = &i
				break
			}
		}

		By("Triggering switchover in replica MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, replicaKey, replicaMdb); err != nil {
				return false
			}
			replicaMdb.Spec.Replication.Primary.PodIndex = newPrimaryPodIndex
			return k8sClient.Update(testCtx, replicaMdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting replica MariaDB to be switched over eventually")
		expectMariadbFn(testCtx, k8sClient, replicaKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			if mdb.Status.CurrentPrimaryPodIndex == nil {
				return false
			}
			return mdb.IsReady() && *mdb.Status.CurrentPrimaryPodIndex != *currentPrimaryPodIndex
		})

		By("Expecting primary MariaDB to be ready")
		expectMariadbFn(testCtx, k8sClient, primaryKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsReady()
		})
	})

	It("should have valid replication status after switchover", testReplicationStatus)
})

var _ = Describe("MariaDB multi-cluster with replication and MaxScale", Ordered, Focus, func() {
	BeforeAll(func() {
		primaryBackup = buildPhysicalBackupWithS3Storage(
			primaryKey,
			"test-multi-cluster",
			primaryKey.Name,
		)(primaryKey)

		primaryMdb = applyDecoratorChain(
			multiClusterMariaDBBuilder(
				prefixedIPAddr(".1.10"),
				multiCluster,
			),
			multiClusterReplicationDecorator(
				primaryGtidDomainId,
				primaryServerStartIndex,
			),
		)(primaryKey)
		replicaMdb = applyDecoratorChain(
			multiClusterMariaDBBuilder(
				prefixedIPAddr(".1.15"),
				multiCluster,
			),
			multiClusterReplicationDecorator(
				replicaGtidDomainId,
				replicaServerStartIndex,
			),
			mariadbBootstrapFromDecorator(
				&mariadbv1alpha1.BootstrapFrom{
					BackupContentType: mariadbv1alpha1.BackupContentTypePhysical,
					S3:                primaryBackup.Spec.Storage.S3,
				},
			),
		)(replicaKey)

		primaryMaxScale = buildMultiClusterMaxScale(primaryMaxScaleKey, prefixedIPAddr(".1.20"))
		replicaMaxScale = buildMultiClusterMaxScale(replicaMaxScaleKey, prefixedIPAddr(".1.24"))

		primaryExternalMdb = buildMultiClusterExternalMariadb(
			primaryKey,
			statefulset.ServiceFQDN(metav1.ObjectMeta{
				Name:      primaryMaxScaleKey.Name,
				Namespace: primaryMaxScaleKey.Namespace,
			}),
		)
		replicaExternalMdb = buildMultiClusterExternalMariadb(
			replicaKey,
			statefulset.ServiceFQDN(metav1.ObjectMeta{
				Name:      replicaMaxScaleKey.Name,
				Namespace: replicaMaxScaleKey.Namespace,
			}),
		)
	})

	AfterAll(func() {
		deletePhysicalBackup(primaryKey, true)
		deleteMariadb(primaryKey, true)
		deleteMaxScale(primaryMaxScaleKey, true)
		deleteExternalMariadb(primaryKey)

		deleteMariadb(replicaKey, true)
		deleteMaxScale(replicaMaxScaleKey, true)
		deleteExternalMariadb(replicaKey)
	})

	It("should reconcile primary cluster", func() {
		By("Creating primary MariaDB")
		Expect(k8sClient.Create(testCtx, primaryMdb)).To(Succeed())

		By("Expecting primary MariaDB to be ready eventually")
		expectMariadbFn(testCtx, k8sClient, primaryKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsReady()
		})

		By("Confiruging primary MariaDB with MaxScale")
		testMaxscale(primaryMdb, primaryMaxScale)

		By("Creating primary ExternalMariaDB")
		Expect(k8sClient.Create(testCtx, primaryExternalMdb)).To(Succeed())

		By("Expecting primary ExternalMariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, primaryKey, primaryExternalMdb); err != nil {
				return false
			}
			return primaryExternalMdb.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should create a physical backup of primary cluster", func() {
		By("Creating PhysicalBackup")
		Expect(k8sClient.Create(testCtx, primaryBackup)).To(Succeed())

		By("Expecting PhysicalBackup to complete eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, primaryKey, primaryBackup); err != nil {
				return false
			}
			return primaryBackup.IsComplete()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should create replica cluster from physical backup", func() {
		By("Creating replica MariaDB")
		Expect(k8sClient.Create(testCtx, replicaMdb)).To(Succeed())

		By("Expecting replica MariaDB to be ready eventually")
		expectMariadbFn(testCtx, k8sClient, replicaKey, func(mdb *mariadbv1alpha1.MariaDB) bool {
			return mdb.IsReady()
		})

		By("Confiruging replica MariaDB with MaxScale")
		testMaxscale(primaryMdb, primaryMaxScale)

		By("Creating replica ExternalMariaDB")
		Expect(k8sClient.Create(testCtx, replicaExternalMdb)).To(Succeed())

		By("Expecting ExternalMariaDB to eventually be ready")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, replicaKey, replicaExternalMdb); err != nil {
				return false
			}
			return replicaExternalMdb.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should allow to perform writes in the primary", func() {
		By("Expecting primary MaxScale to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, primaryMaxScaleKey, primaryMaxScale); err != nil {
				return false
			}
			return primaryMaxScale.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		caCert, err := testRefResolver.SecretKeyRef(
			testCtx,
			mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: "mariadb-server-ca",
				},
				Key: "ca.crt",
			},
			testNamespace,
		)
		Expect(err).ToNot(HaveOccurred())

		client, err := sql.NewClient(
			sql.WithHost(statefulset.ServiceFQDN(metav1.ObjectMeta{
				Name:      primaryMaxScaleKey.Name,
				Namespace: primaryMaxScaleKey.Namespace,
			})),
			sql.WithPort(3306),
			sql.WithUsername("root"),
			sql.WithPassword("MariaDB11!"),
			sql.WithCustomTLSCA("mariadb-server-ca", []byte(caCert)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer client.Close()

		// in addition to verify writes, this increases gtid_current_pos, to be validated in the next step.
		By("Writing in primary MaxScale")
		query := `CREATE DATABASE IF NOT EXISTS test;`
		Expect(client.Exec(testCtx, query)).To(Succeed())
		query = `CREATE TABLE IF NOT EXISTS test.test (id INT PRIMARY KEY AUTO_INCREMENT, test VARCHAR(100));`
		Expect(client.Exec(testCtx, query)).To(Succeed())
		query = `INSERT INTO test.test (test) VALUES ('test');`
		Expect(client.Exec(testCtx, query)).To(Succeed())
	})

	It("should have valid replication status", testReplicationStatus)
})

func multiClusterMariaDBBuilder(ipAddr string,
	multiCluster mariadbv1alpha1.MultiCluster) func(key types.NamespacedName) *mariadbv1alpha1.MariaDB {
	return func(key types.NamespacedName) *mariadbv1alpha1.MariaDB {
		mdb := &mariadbv1alpha1.MariaDB{
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
				Replicas: 2,
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
				PrimaryService: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": ipAddr,
						},
					},
				},
				TLS: &mariadbv1alpha1.TLS{
					Enabled:  true,
					Required: ptr.To(true),
					ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
						Name: "mariadb-server-ca",
					},
					ServerCertAdditionalNames: []string{
						ipAddr,
					},
					ClientCASecretRef: &mariadbv1alpha1.LocalObjectReference{
						Name: "mariadb-server-ca",
					},
				},
				MultiCluster: &multiCluster,
			},
		}
		return applyMariadbTestConfig(mdb)
	}
}

func multiClusterReplicationDecorator(gtidDomainID *int, serverStartIndex *int) func(*mariadbv1alpha1.MariaDB) *mariadbv1alpha1.MariaDB {
	return func(mdb *mariadbv1alpha1.MariaDB) *mariadbv1alpha1.MariaDB {
		mdb.Spec.Replication = &mariadbv1alpha1.Replication{
			Enabled: true,
			ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
				GtidDomainID:       gtidDomainID,
				ServerIDStartIndex: serverStartIndex,
				SemiSyncEnabled:    ptr.To(false),
				Replica: mariadbv1alpha1.ReplicaReplication{
					ReplPasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: testPwdKey.Name,
							},
							Key: testPwdSecretKey,
						},
					},
				},
			},
		}
		return mdb
	}
}

func mariadbBootstrapFromDecorator(bootSstrapFrom *mariadbv1alpha1.BootstrapFrom) func(*mariadbv1alpha1.MariaDB) *mariadbv1alpha1.MariaDB {
	return func(mdb *mariadbv1alpha1.MariaDB) *mariadbv1alpha1.MariaDB {
		mdb.Spec.BootstrapFrom = bootSstrapFrom
		return mdb
	}
}

func buildMultiClusterExternalMariadb(key types.NamespacedName, host string) *mariadbv1alpha1.ExternalMariaDB {
	return &mariadbv1alpha1.ExternalMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.ExternalMariaDBSpec{
			Host:     host,
			Username: ptr.To("root"),
			PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: testPwdKey.Name,
				},
				Key: testPwdSecretKey,
			},
			TLS: &mariadbv1alpha1.ExternalTLS{
				TLS: mariadbv1alpha1.TLS{
					Enabled: true,
					ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
						Name: "mariadb-server-ca",
					},
				},
			},
		},
	}
}

func buildMultiClusterMaxScale(key types.NamespacedName, ipAddr string) *mariadbv1alpha1.MaxScale {
	mxs := &mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.MaxScaleSpec{
			Replicas: 2,
			KubernetesService: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": ipAddr,
					},
				},
			},
			Services: []mariadbv1alpha1.MaxScaleService{
				{
					Name:   "rw-router",
					Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
					Listener: mariadbv1alpha1.MaxScaleListener{
						Port: 3306,
					},
					Params: map[string]string{
						"enable_root_user": "true",
					},
				},
			},
			Monitor: mariadbv1alpha1.MaxScaleMonitor{
				Name:                  key.Name,
				CooperativeMonitoring: ptr.To(mariadbv1alpha1.CooperativeMonitoringMajorityOfRunning),
			},
			Auth: mariadbv1alpha1.MaxScaleAuth{
				Generate:      ptr.To(false),
				AdminUsername: "mariadb-operator",
				AdminPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
				},
				ClientUsername: "root",
				ClientPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
				},
				ServerUsername: "root",
				ServerPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
				},
				MonitorUsername: "root",
				MonitorPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
				},
				SyncUsername: ptr.To("root"),
				SyncPasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
				},
			},
			TLS: &mariadbv1alpha1.MaxScaleTLS{
				Enabled: true,
				ListenerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
					Name: "mariadb-server-ca",
				},
				ServerCASecretRef: &mariadbv1alpha1.LocalObjectReference{
					Name: "mariadb-server-ca",
				},
			},
		},
	}
	return applyMaxscaleTestConfig(mxs)
}

func testReplicationStatus() {
	By("Getting primary MariaDB client")
	Expect(k8sClient.Get(testCtx, primaryKey, primaryMdb)).To(Succeed())
	Expect(primaryMdb.Status.CurrentPrimaryPodIndex).NotTo(BeNil())

	// replica index in primary cluster
	var primaryPodIndex *int
	for i := 0; i < int(primaryMdb.Spec.Replicas); i++ {
		if i != *primaryMdb.Status.CurrentPrimaryPodIndex {
			primaryPodIndex = &i
			break
		}
	}
	Expect(primaryPodIndex).NotTo(BeNil())
	primaryClient, err := sql.NewInternalClientWithPodIndex(testCtx, primaryMdb, testRefResolver, *primaryPodIndex)
	Expect(err).To(Succeed())
	defer primaryClient.Close()

	By("Ensuring valid primary gtid_current_pos")
	testGtidCurrentPos(*primaryClient, *primaryGtidDomainId)
	By("Ensuring primary replication running")
	testReplicationRunning(*primaryClient, nil)

	By("Getting primary replica MariaDB client")
	Expect(k8sClient.Get(testCtx, replicaKey, replicaMdb)).To(Succeed())
	Expect(replicaMdb.Status.CurrentPrimary).NotTo(BeNil())

	// primary index in replica cluster
	primaryReplicaPodIndex := replicaMdb.Status.CurrentPrimaryPodIndex
	primaryReplicaClient, err := sql.NewInternalClientWithPodIndex(testCtx, replicaMdb, testRefResolver, *primaryReplicaPodIndex)
	Expect(err).To(Succeed())
	defer primaryReplicaClient.Close()

	By("Ensuring valid primary replica gtid_current_pos")
	testGtidCurrentPos(*primaryReplicaClient, *primaryGtidDomainId, *replicaGtidDomainId)
	By("Ensuring primary replica replication running")
	testReplicationRunning(*primaryReplicaClient, &replicationctrl.MultiClusterReplicaConnectionName)

	By("Getting replica MariaDB client")
	Expect(k8sClient.Get(testCtx, replicaKey, replicaMdb)).To(Succeed())
	Expect(replicaMdb.Status.CurrentPrimary).NotTo(BeNil())

	// replica index in replica cluster
	var replicaPodIndex *int
	for i := 0; i < int(replicaMdb.Spec.Replicas); i++ {
		if i != *replicaMdb.Status.CurrentPrimaryPodIndex {
			replicaPodIndex = &i
			break
		}
	}
	Expect(replicaPodIndex).ToNot(BeNil())
	replicaClient, err := sql.NewInternalClientWithPodIndex(testCtx, replicaMdb, testRefResolver, *replicaPodIndex)
	Expect(err).To(Succeed())
	defer replicaClient.Close()

	By("Ensuring valid replica gtid_current_pos")
	testGtidCurrentPos(*replicaClient, *primaryGtidDomainId, *replicaGtidDomainId)
	By("Ensuring replica replication running")
	testReplicationRunning(*replicaClient, nil)
}

func testGtidCurrentPos(client sql.Client, domainIds ...int) {
	Eventually(func(g Gomega) bool {
		rawGtid, err := client.GtidCurrentPos(testCtx)
		g.Expect(err).ToNot(HaveOccurred())

		for domainId := range domainIds {
			gtid, err := replication.ParseGtidWithDomainId(rawGtid, uint32(domainId), testLogger)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gtid).ToNot(BeNil())
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())
}

func testReplicationRunning(client sql.Client, connectionName *string) {
	var opts []sql.ReplicationOpt
	if connectionName != nil {
		opts = append(opts, sql.WithConnectionName(*connectionName))
	}

	Eventually(func(g Gomega) bool {
		status, err := client.ReplicaStatus(testCtx, testLogger, opts...)
		Expect(err).To(Succeed())

		return ptr.Deref(status.SlaveIORunning, false) &&
			ptr.Deref(status.SlaveSQLRunning, false) &&
			ptr.Deref(status.LastSQLErrno, -1) == 0 &&
			ptr.Deref(status.LastIOErrno, -1) == 0 &&
			ptr.Deref(status.SecondsBehindMaster, -1) == 0
	}, testTimeout, testInterval).Should(BeTrue())
}
