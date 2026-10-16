package controller

import (
	"fmt"
	"strconv"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// A primary that loses its network for a minute is failed over by MaxScale, and the operator demotes it once it
// is reachable again. On 2026-10-16 the demotion ran RESET MASTER first: with the binary log gone, the demoted
// primary fell back to the gtid_slave_pos recorded when its primary configuration had last been reconciled, eleven
// minutes earlier, replayed transactions it already held and stopped on duplicate keys. Seven old primaries were
// then wiped and rebuilt. The demoted primary must resume from its own position instead.
var _ = Describe("MariaDB demoted primary rejoin", Ordered, func() {
	var (
		key = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		mdb *mariadbv1alpha1.MariaDB
	)

	const (
		rejoinDB       = "rejoindb"
		rejoinTable    = "orders"
		oldPrimaryIdx  = 0
		newPrimaryIdx  = 1
		suspendSettle  = 5 * time.Second
		beforeStale    = 50
		afterStale     = 100
		afterFailover  = 150
		rowsPerInsert  = 50
		rowValueFormat = "order-%d"
	)

	updateMariaDB := func(mutate func(mdb *mariadbv1alpha1.MariaDB)) {
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			mutate(mdb)
			g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	}

	systemVariableInPod := func(podIndex int, variable string) string {
		client := sqlClientForTestPod(mdb, podIndex)
		defer client.Close()
		value, err := client.SystemVariable(testCtx, variable)
		Expect(err).ToNot(HaveOccurred())
		return value
	}

	insertRows := func(podIndex, from int) {
		values := ""
		for i := from; i < from+rowsPerInsert; i++ {
			if values != "" {
				values += ","
			}
			values += fmt.Sprintf("(%d,'%s')", i, fmt.Sprintf(rowValueFormat, i))
		}
		executeSqlInPodByIndex(mdb, podIndex, fmt.Sprintf("INSERT INTO %s.%s (id, v) VALUES %s;", rejoinDB, rejoinTable, values))
	}

	rowCountInPod := func(podIndex int) (int, error) {
		clientSet := sql.NewClientSet(mdb, refresolver.New(k8sClient))
		client, err := clientSet.ClientForIndex(testCtx, podIndex)
		if err != nil {
			return 0, err
		}
		defer client.Close()
		row, err := client.QueryColumnMap(testCtx, fmt.Sprintf("SELECT COUNT(*) AS c FROM %s.%s;", rejoinDB, rejoinTable))
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(row["c"])
	}

	expectRowCount := func(podIndex, count int) {
		Eventually(func() int {
			c, err := rowCountInPod(podIndex)
			if err != nil {
				return -1
			}
			return c
		}, testHighTimeout, testInterval).Should(Equal(count), "row count on Pod %d", podIndex)
	}

	BeforeAll(func() {
		mdb = buildTestMariaDBWithRepl(key)
		mdb.Spec.Replicas = 2
		mdb.Spec.Replication.Primary.AutoFailover = ptr.To(false)
		applyMariadbTestConfig(mdb)
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, true)
		})
		Eventually(func() bool {
			return k8sClient.Get(testCtx, key, mdb) == nil && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())
	})

	It("resumes the demoted primary from its own position without recovering it", func() {
		By("Writing on the primary")
		executeSqlInPodByIndex(mdb, oldPrimaryIdx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", rejoinDB))
		executeSqlInPodByIndex(mdb, oldPrimaryIdx,
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY, v VARCHAR(32));", rejoinDB, rejoinTable))
		insertRows(oldPrimaryIdx, 0)
		expectRowCount(newPrimaryIdx, beforeStale)

		By("Recording a gtid_slave_pos on the primary, as a primary configuration reconciliation does")
		stalePos := systemVariableInPod(oldPrimaryIdx, "gtid_binlog_pos")
		executeSqlInPodByIndex(mdb, oldPrimaryIdx, fmt.Sprintf("SET GLOBAL gtid_slave_pos='%s';", stalePos))

		By("Writing more on the primary, which leaves that gtid_slave_pos behind")
		insertRows(oldPrimaryIdx, beforeStale)
		expectRowCount(newPrimaryIdx, afterStale)
		ownPos := systemVariableInPod(oldPrimaryIdx, "gtid_binlog_pos")
		Expect(ownPos).ToNot(Equal(stalePos))

		By("Suspending reconciliation to stage a MaxScale failover")
		updateMariaDB(func(mdb *mariadbv1alpha1.MariaDB) {
			mdb.Spec.Suspend = true
		})
		time.Sleep(suspendSettle)

		By("Promoting the replica the way MaxScale does, leaving the old primary writable")
		executeSqlInPodByIndex(mdb, newPrimaryIdx, "STOP SLAVE;")
		executeSqlInPodByIndex(mdb, newPrimaryIdx, "RESET SLAVE ALL;")
		executeSqlInPodByIndex(mdb, newPrimaryIdx, "SET GLOBAL read_only=0;")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			mdb.Status.UpdateCurrentPrimary(mdb, newPrimaryIdx)
			g.Expect(k8sClient.Status().Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())
		updateMariaDB(func(mdb *mariadbv1alpha1.MariaDB) {
			mdb.Spec.Replication.Primary.PodIndex = ptr.To(newPrimaryIdx)
		})

		By("Writing on the new primary")
		insertRows(newPrimaryIdx, afterStale)

		By("Resuming reconciliation")
		updateMariaDB(func(mdb *mariadbv1alpha1.MariaDB) {
			mdb.Spec.Suspend = false
		})

		By("Expecting the old primary to be demoted and to replay only the new primary writes")
		Eventually(func() bool {
			isReplica, err := replicaInPodByIndex(mdb, oldPrimaryIdx)
			return err == nil && isReplica
		}, testHighTimeout, testInterval).Should(BeTrue())
		expectRowCount(oldPrimaryIdx, afterFailover)

		By("Expecting healthy replication threads without errors")
		Eventually(func(g Gomega) bool {
			client := sqlClientForTestPod(mdb, oldPrimaryIdx)
			defer client.Close()
			status, err := client.ReplicaStatus(testCtx, GinkgoLogr)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(status.IsIOThreadRunning()).To(BeTrue())
			g.Expect(status.IsSQLThreadRunning()).To(BeTrue())
			g.Expect(ptr.Deref(status.LastSQLErrno, 0)).To(Equal(0))
			g.Expect(ptr.Deref(status.LastIOErrno, 0)).To(Equal(0))
			return true
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting the old primary binary log to be kept")
		Expect(systemVariableInPod(oldPrimaryIdx, "gtid_binlog_pos")).ToNot(BeEmpty())

		By("Expecting no replica recovery")
		Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
		Expect(mdb.IsRecoveringReplicas()).To(BeFalse())
		Expect(mdb.ReplicaRecoveryError()).To(Succeed())
		Expect(getActiveReplicaRecoveryReplicas(mdb)).To(BeEmpty())
	})
})

func sqlClientForTestPod(mdb *mariadbv1alpha1.MariaDB, podIndex int) *sql.Client {
	clientSet := sql.NewClientSet(mdb, refresolver.New(k8sClient))
	client, err := clientSet.ClientForIndex(testCtx, podIndex)
	Expect(err).ToNot(HaveOccurred())
	return client
}
