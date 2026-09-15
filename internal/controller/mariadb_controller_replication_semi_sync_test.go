package controller

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

const (
	// testSemiSyncAckTimeout is deliberately far longer than the time a healthy replica needs to acknowledge, so that a
	// regression is a hard failure instead of a silent delay: a node armed as a semi-synchronous primary with no
	// replicas connected to it blocks every binlogged write it performs for this whole duration.
	testSemiSyncAckTimeout = 100 * time.Hour
	// testSemiSyncWriteBudget is the time budget for a binlogged write in the primary. It sits well above the
	// sub-second round trip of a healthy acknowledgement and well below testSemiSyncAckTimeout.
	testSemiSyncWriteBudget = 15 * time.Second
	// testSemiSyncBucket is provisioned by 'make install-minio', see 'hack/config/minio.yaml'.
	testSemiSyncBucket = "test-semi-sync"
	// testSemiSyncRootPassword and testSemiSyncRootPasswordRotated back the root password rotation spec. This Describe
	// owns its root password Secret instead of sharing the suite-wide one, so that rotating it does not change the root
	// password of every other MariaDB in the suite.
	testSemiSyncRootPassword        = "MariaDB11!"
	testSemiSyncRootPasswordRotated = "MariaDB22!"
)

var _ = Describe("MariaDB replication semi-sync", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		rootPwdKey = types.NamespacedName{
			Name:      "mariadb-repl-semi-sync-root",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	// primaryPodIndex refreshes the MariaDB and returns the Pod index the operator currently considers the primary.
	primaryPodIndex := func() int {
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			return mdb.IsReady() && mdb.Status.CurrentPrimaryPodIndex != nil
		}, testHighTimeout, testInterval).Should(BeTrue())
		return *mdb.Status.CurrentPrimaryPodIndex
	}

	BeforeAll(func() {
		mdb = buildTestMariaDBWithRepl(key)
		applyMariadbTestConfig(mdb)

		mdb.Spec.Replication.SemiSyncEnabled = ptr.To(true)
		mdb.Spec.Replication.SemiSyncAckTimeout = &metav1.Duration{Duration: testSemiSyncAckTimeout}
		// The server default, set explicitly: it is the setting that turns a wrongly armed node into a stalled one,
		// which is precisely what these specs must detect.
		mdb.Spec.Replication.SemiSyncWaitNoSlave = ptr.To(true)
		// Opt in to booting unarmed and read_only, so a node is never writable while it is unable to require an
		// acknowledgement. Without it the nodes boot as writable semi-synchronous primaries and the assertions below
		// would only hold from the first reconciliation onwards.
		mdb.Spec.Replication.SemiSyncBootAsReplica = ptr.To(true)

		By("Creating a dedicated root password Secret")
		rootPwdSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rootPwdKey.Name,
				Namespace: rootPwdKey.Namespace,
				// Without this label the operator does not watch the Secret, so rotating it would not trigger a
				// reconciliation and the spec below would time out waiting for a rotation that never starts.
				Labels: map[string]string{
					metadata.WatchLabel: "",
				},
			},
			Data: map[string][]byte{
				testPwdSecretKey: []byte(testSemiSyncRootPassword),
			},
		}
		Expect(k8sClient.Create(testCtx, &rootPwdSecret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &rootPwdSecret)).To(Succeed())
		})

		mdb.Spec.RootPasswordSecretKeyRef = mariadbv1alpha1.GeneratedSecretKeyRef{
			SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: rootPwdKey.Name,
				},
				Key: testPwdSecretKey,
			},
		}

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

	It("should enable semi-sync in the primary only", func() {
		primary := primaryPodIndex()

		expectSemiSyncTopology(mdb, primary)
		expectSemiSyncPrimaryAcknowledged(mdb, primary)
		expectReplicationConnectionsReady(mdb)
	})

	It("should not stall binlogged writes in the primary", func() {
		expectSemiSyncWriteNotStalled(mdb, primaryPodIndex())
	})

	// SQL resources are reconciled with CREATE USER / GRANT / CREATE DATABASE statements, which are binlogged and
	// therefore wait for a semi-synchronous acknowledgement. They never become ready against a stalled primary.
	It("should reconcile SQL resources", func() {
		sqlKey := types.NamespacedName{
			Name:      "mariadb-repl-semi-sync",
			Namespace: testNamespace,
		}

		database := mariadbv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sqlKey.Name,
				Namespace: sqlKey.Namespace,
			},
			Spec: mariadbv1alpha1.DatabaseSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: key.Name,
					},
					WaitForIt: true,
				},
				CharacterSet: "utf8",
				Collate:      "utf8_general_ci",
			},
		}
		user := mariadbv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sqlKey.Name,
				Namespace: sqlKey.Namespace,
			},
			Spec: mariadbv1alpha1.UserSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: key.Name,
					},
					WaitForIt: true,
				},
				PasswordSecretKeyRef: &testPasswordSecretRef,
				MaxUserConnections:   20,
			},
		}
		grant := mariadbv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sqlKey.Name,
				Namespace: sqlKey.Namespace,
			},
			Spec: mariadbv1alpha1.GrantSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: key.Name,
					},
					WaitForIt: true,
				},
				Privileges: []string{
					"SELECT",
					"INSERT",
				},
				Database: sqlKey.Name,
				Table:    "*",
				Username: sqlKey.Name,
			},
		}

		By("Creating SQL resources")
		Expect(k8sClient.Create(testCtx, &database)).To(Succeed())
		Expect(k8sClient.Create(testCtx, &user)).To(Succeed())
		Expect(k8sClient.Create(testCtx, &grant)).To(Succeed())
		// The Grant is revoked first: its finalizer issues a REVOKE against the User, which must still exist.
		DeferCleanup(func() {
			By("Deleting Grant")
			Expect(k8sClient.Delete(testCtx, &grant)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(testCtx, sqlKey, &grant))
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting User")
			Expect(k8sClient.Delete(testCtx, &user)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(testCtx, sqlKey, &user))
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Database")
			Expect(k8sClient.Delete(testCtx, &database)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(testCtx, sqlKey, &database))
			}, testTimeout, testInterval).Should(BeTrue())
		})

		By("Expecting Database to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, sqlKey, &database)).To(Succeed())
			return database.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting User to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, sqlKey, &user)).To(Succeed())
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Grant to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, sqlKey, &grant)).To(Succeed())
			return grant.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	// A logical backup takes a read lock and dumps the database through the same Pods. It never completes against a
	// stalled primary.
	It("should complete a logical backup", func() {
		backupKey := types.NamespacedName{
			Name:      "mariadb-repl-semi-sync-backup",
			Namespace: testNamespace,
		}
		backup := getBackupWithPVCStorage(backupKey)
		backup.Spec.MariaDBRef.Name = key.Name

		By("Creating Backup")
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
		})

		By("Expecting Backup to complete eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, backupKey, backup)).To(Succeed())
			return backup.IsComplete()
		}, testHighTimeout, testInterval).Should(BeTrue())

		expectSemiSyncTopology(mdb, primaryPodIndex())
	})

	It("should complete a logical backup and restore with S3 storage", func() {
		backupKey := types.NamespacedName{
			Name:      "mariadb-repl-semi-sync-backup-s3",
			Namespace: testNamespace,
		}
		backup := getBackupWithS3Storage(backupKey, testSemiSyncBucket, "")
		backup.Spec.MariaDBRef.Name = key.Name

		By("Creating Backup")
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
		})

		By("Expecting Backup to complete eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, backupKey, backup)).To(Succeed())
			return backup.IsComplete()
		}, testHighTimeout, testInterval).Should(BeTrue())

		restoreKey := types.NamespacedName{
			Name:      "mariadb-repl-semi-sync-restore-s3",
			Namespace: testNamespace,
		}
		restore := mariadbv1alpha1.Restore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      restoreKey.Name,
				Namespace: restoreKey.Namespace,
			},
			Spec: mariadbv1alpha1.RestoreSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
						Name: key.Name,
					},
					WaitForIt: true,
				},
				RestoreSource: mariadbv1alpha1.RestoreSource{
					S3: getS3Storage(testSemiSyncBucket, ""),
				},
			},
		}

		By("Creating Restore")
		Expect(k8sClient.Create(testCtx, &restore)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, &restore)).To(Succeed())
		})

		By("Expecting Restore to complete eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, restoreKey, &restore)).To(Succeed())
			return restore.IsComplete()
		}, testHighTimeout, testInterval).Should(BeTrue())

		primary := primaryPodIndex()
		expectSemiSyncTopology(mdb, primary)
		expectSemiSyncPrimaryAcknowledged(mdb, primary)
		expectSemiSyncWriteNotStalled(mdb, primary)
	})

	// A Pod always boots with rpl_semi_sync_master_enabled=OFF, since that is what the configuration file renders.
	// A restarted replica must therefore come back disarmed without the operator having to do anything, and the
	// primary must stay armed throughout.
	It("should keep semi-sync consistent across a replica restart", func() {
		primary := primaryPodIndex()
		var replica int
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			if i != primary {
				replica = i
				break
			}
		}

		By(fmt.Sprintf("Tearing down replica Pod '%d'", replica))
		deletePodByIndex(mdb, replica)

		By("Expecting MariaDB to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			return mdb.IsReady()
		}, testHighTimeout, testInterval).MustPassRepeatedly(10).Should(BeTrue())

		expectSemiSyncTopology(mdb, primaryPodIndex())
		expectSemiSyncPrimaryAcknowledged(mdb, primaryPodIndex())
		expectSemiSyncWriteNotStalled(mdb, primaryPodIndex())
	})

	// Switchover arms the promoted node and disarms the demoted one. Switching back exercises the reverse transition
	// on nodes that already served as primary.
	It("should move semi-sync on switchover", func() {
		originalPrimary := primaryPodIndex()
		var newPrimary int
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			if i != originalPrimary {
				newPrimary = i
				break
			}
		}

		switchPrimaryTo := func(podIndex int) {
			By(fmt.Sprintf("Expecting MariaDB to eventually update primary to index '%d'", podIndex))
			Eventually(func(g Gomega) bool {
				g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
				mdb.Spec.Replication.Primary.PodIndex = ptr.To(podIndex)
				g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By(fmt.Sprintf("Expecting MariaDB to eventually change primary to index '%d'", podIndex))
			Eventually(func(g Gomega) bool {
				g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
				return mdb.IsReady() && mdb.Status.CurrentPrimaryPodIndex != nil && *mdb.Status.CurrentPrimaryPodIndex == podIndex
			}, testHighTimeout, testInterval).Should(BeTrue())

			expectSemiSyncTopology(mdb, podIndex)
			expectSemiSyncPrimaryAcknowledged(mdb, podIndex)
			expectSemiSyncWriteNotStalled(mdb, podIndex)
			expectReplicationConnectionsReady(mdb)
		}

		switchPrimaryTo(newPrimary)
		switchPrimaryTo(originalPrimary)
	})

	// Failover promotes a replica without the demoted primary taking part: the promoted node must end up armed, and
	// the old primary must come back as a disarmed replica once its Pod is recreated.
	It("should move semi-sync on failover", func() {
		originalPrimary := primaryPodIndex()

		By(fmt.Sprintf("Tearing down primary Pod '%d'", originalPrimary))
		deletePodByIndex(mdb, originalPrimary)

		By("Expecting MariaDB to eventually change primary")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			if !mdb.IsReady() || mdb.Status.CurrentPrimaryPodIndex == nil {
				return false
			}
			return *mdb.Status.CurrentPrimaryPodIndex != originalPrimary
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			return mdb.IsReady()
		}, testHighTimeout, testInterval).MustPassRepeatedly(10).Should(BeTrue())

		newPrimary := primaryPodIndex()
		Expect(newPrimary).ToNot(Equal(originalPrimary))

		expectSemiSyncTopology(mdb, newPrimary)
		expectSemiSyncPrimaryAcknowledged(mdb, newPrimary)
		expectSemiSyncWriteNotStalled(mdb, newPrimary)
		expectReplicationConnectionsReady(mdb)
	})

	// Rotating the root password runs ALTER USER in the primary, which is binlogged and therefore waits for a
	// semi-synchronous acknowledgement: against a wrongly armed primary it never returns, and the rotation never
	// completes. It is also the one flow where the operator cannot authenticate for a while, because
	// 'spec.rootPasswordSecretKeyRef' already holds the new password while the servers still have the old one until the
	// 'Root Password' phase runs, so it exercises how the semi-sync reconciliation behaves against unreachable Pods.
	//
	// It runs last in this Ordered container because it leaves the MariaDB with a different root password.
	It("should rotate the root password", func() {
		By("Expecting the root password hash to be set")
		var previousHash string
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			if mdb.Status.RootPasswordHash == nil {
				return false
			}
			previousHash = *mdb.Status.RootPasswordHash
			return true
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Rotating the root password Secret")
		Eventually(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(testCtx, rootPwdKey, &secret)).To(Succeed())
			secret.Data[testPwdSecretKey] = []byte(testSemiSyncRootPasswordRotated)
			g.Expect(k8sClient.Update(testCtx, &secret)).To(Succeed())
		}, testTimeout, testInterval).Should(Succeed())

		By("Expecting the root password hash to change eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			return mdb.Status.RootPasswordHash != nil && *mdb.Status.RootPasswordHash != previousHash
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			return mdb.IsReady()
		}, testHighTimeout, testInterval).MustPassRepeatedly(10).Should(BeTrue())

		// 'sql.Connect' pings on connect, so building a client is itself the assertion that the rotated password is the
		// one the server now accepts: the clients resolve 'spec.rootPasswordSecretKeyRef', which holds the new value.
		By("Expecting to connect to every Pod with the rotated root password")
		Eventually(func(g Gomega) bool {
			for i := 0; i < int(mdb.Spec.Replicas); i++ {
				withSemiSyncSqlClient(g, mdb, i, func(client *sql.Client) {})
			}
			return true
		}, testHighTimeout, testInterval).Should(BeTrue())

		primary := primaryPodIndex()
		expectSemiSyncTopology(mdb, primary)
		expectSemiSyncPrimaryAcknowledged(mdb, primary)
		expectSemiSyncWriteNotStalled(mdb, primary)
		expectReplicationConnectionsReady(mdb)
	})
})

// withSemiSyncSqlClient runs fn against a SQL client connected to a given Pod. Pass the Gomega instance provided by
// Eventually when polling, or Default when asserting once.
func withSemiSyncSqlClient(g Gomega, mdb *mariadbv1alpha1.MariaDB, podIndex int, fn func(client *sql.Client)) {
	clientSet := sql.NewClientSet(mdb, testRefResolver)
	defer clientSet.Close()

	client, err := clientSet.ClientForIndex(testCtx, podIndex)
	g.Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Could not create a client for Pod '%d'.", podIndex))

	fn(client)
}

// expectSemiSyncTopology asserts that primary-side semi-sync converges to the role of every Pod: enabled in the
// primary and disabled in the replicas. Replica-side semi-sync is rendered in the configuration file and must stay
// enabled everywhere, so that any node can be promoted and acknowledge without reconfiguration.
func expectSemiSyncTopology(mdb *mariadbv1alpha1.MariaDB, primaryPodIndex int) {
	By(fmt.Sprintf("Expecting primary-side semi-sync to be enabled in Pod '%d' only", primaryPodIndex))
	Eventually(func(g Gomega) bool {
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			shouldBeEnabled := i == primaryPodIndex

			withSemiSyncSqlClient(g, mdb, i, func(client *sql.Client) {
				masterEnabled, err := client.IsSemiSyncMasterEnabled(testCtx)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(masterEnabled).To(Equal(shouldBeEnabled),
					fmt.Sprintf("Unexpected rpl_semi_sync_master_enabled in Pod '%d'.", i))

				slaveEnabled, err := client.IsSystemVariableEnabled(testCtx, "rpl_semi_sync_slave_enabled")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(slaveEnabled).To(BeTrue(),
					fmt.Sprintf("Unexpected rpl_semi_sync_slave_enabled in Pod '%d'.", i))
			})
		}
		return true
	}, testHighTimeout, testInterval).Should(BeTrue())
}

// expectSemiSyncPrimaryAcknowledged asserts that the primary is armed and that every replica is connected to it as a
// semi-synchronous acknowledger. Without this, semi-sync could be reported as enabled while no replica can ever
// acknowledge, which is the stall this feature prevents.
func expectSemiSyncPrimaryAcknowledged(mdb *mariadbv1alpha1.MariaDB, primaryPodIndex int) {
	By(fmt.Sprintf("Expecting the primary in Pod '%d' to be acknowledged by all its replicas", primaryPodIndex))
	Eventually(func(g Gomega) bool {
		var status string
		var clients int

		withSemiSyncSqlClient(g, mdb, primaryPodIndex, func(client *sql.Client) {
			var err error
			status, err = client.StatusVariable(testCtx, "Rpl_semi_sync_master_status")
			g.Expect(err).ToNot(HaveOccurred())

			clients, err = client.StatusVariableInt(testCtx, "Rpl_semi_sync_master_clients")
			g.Expect(err).ToNot(HaveOccurred())
		})
		return status == "ON" && clients == int(mdb.Spec.Replicas)-1
	}, testHighTimeout, testInterval).Should(BeTrue())
}

// expectSemiSyncWriteNotStalled runs binlogged writes in the primary and asserts that they were acknowledged by a
// replica rather than left waiting for the whole ACK timeout. Both signals matter: the elapsed time catches a stall,
// and the counters catch writes that completed only because the ACK timeout expired and semi-sync fell back to
// asynchronous.
func expectSemiSyncWriteNotStalled(mdb *mariadbv1alpha1.MariaDB, primaryPodIndex int) {
	table := fmt.Sprintf("semi_sync_probe_%d", time.Now().UnixNano())
	yesTxBefore, noTxBefore := getSemiSyncTxCounters(mdb, primaryPodIndex)

	By(fmt.Sprintf("Expecting binlogged writes in Pod '%d' to complete without stalling", primaryPodIndex))
	start := time.Now()
	executeSqlInPodByIndex(mdb, primaryPodIndex,
		fmt.Sprintf("CREATE TABLE `%s`.`%s` (id INT PRIMARY KEY);", testDatabase, table))
	executeSqlInPodByIndex(mdb, primaryPodIndex,
		fmt.Sprintf("INSERT INTO `%s`.`%s` (id) VALUES (1);", testDatabase, table))
	executeSqlInPodByIndex(mdb, primaryPodIndex,
		fmt.Sprintf("DROP TABLE `%s`.`%s`;", testDatabase, table))
	elapsed := time.Since(start)

	Expect(elapsed).To(BeNumerically("<", testSemiSyncWriteBudget),
		fmt.Sprintf("Binlogged writes in Pod '%d' took %s, they are waiting for an acknowledgement.", primaryPodIndex, elapsed))

	yesTxAfter, noTxAfter := getSemiSyncTxCounters(mdb, primaryPodIndex)
	Expect(yesTxAfter).To(BeNumerically(">", yesTxBefore),
		"Transactions were not acknowledged by any replica.")
	Expect(noTxAfter).To(Equal(noTxBefore),
		"Transactions completed without an acknowledgement.")
}

func getSemiSyncTxCounters(mdb *mariadbv1alpha1.MariaDB, podIndex int) (yesTx, noTx int) {
	withSemiSyncSqlClient(Default, mdb, podIndex, func(client *sql.Client) {
		var err error
		yesTx, err = client.StatusVariableInt(testCtx, "Rpl_semi_sync_master_yes_tx")
		Expect(err).ToNot(HaveOccurred())

		noTx, err = client.StatusVariableInt(testCtx, "Rpl_semi_sync_master_no_tx")
		Expect(err).ToNot(HaveOccurred())
	})
	return yesTx, noTx
}

func expectReplicationConnectionsReady(mdb *mariadbv1alpha1.MariaDB) {
	connKeys := []types.NamespacedName{
		{
			Name:      mdb.Name,
			Namespace: mdb.Namespace,
		},
		mdb.PrimaryConnectioneKey(),
		mdb.SecondaryConnectioneKey(),
	}

	for _, connKey := range connKeys {
		By(fmt.Sprintf("Expecting Connection '%s' to be ready eventually", connKey.Name))
		Eventually(func(g Gomega) bool {
			var conn mariadbv1alpha1.Connection
			g.Expect(k8sClient.Get(testCtx, connKey, &conn)).To(Succeed())
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
	}
}
