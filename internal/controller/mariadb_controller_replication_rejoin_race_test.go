package controller

import (
	"fmt"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A replica that restarts comes back with read_only OFF while its SQL thread works through a backlog, and the
// operator reconfigures it as drifted. 26.3.8 read the replica position before stopping the threads: the SQL thread
// applied more transactions in between, the older position was written back, and gtid_strict_mode refused to apply
// those transactions again (error 1950) on every reconfiguration from then on (postar-stage, 2026-10-17).
var _ = Describe("MariaDB replica reconfigured while catching up", Ordered, func() {
	var (
		key = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		mdb *mariadbv1alpha1.MariaDB
	)

	const (
		raceDB         = "racedb"
		raceTable      = "events"
		primaryIdx     = 0
		replicaIdx     = 1
		rounds         = 3
		backlogPerTurn = 20000
		reconcileAnno  = "k8s.mariadb.com/test-rejoin-race"
	)

	countIn := func(podIndex int) (int, error) {
		sqlClient := sqlClientForTestPod(mdb, podIndex)
		defer sqlClient.Close()
		row, err := sqlClient.QueryColumnMap(testCtx, fmt.Sprintf("SELECT COUNT(*) AS c FROM %s.%s;", raceDB, raceTable))
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(row["c"])
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

	It("never rewinds the replica position", func() {
		executeSqlInPodByIndex(mdb, primaryIdx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", raceDB))
		executeSqlInPodByIndex(mdb, primaryIdx,
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT AUTO_INCREMENT PRIMARY KEY, v INT);", raceDB, raceTable))

		for round := 1; round <= rounds; round++ {
			By(fmt.Sprintf("Round %d: queueing a backlog the replica has fetched but not applied", round))
			executeSqlInPodByIndex(mdb, replicaIdx, "STOP SLAVE SQL_THREAD;")
			executeSqlInPodByIndex(mdb, primaryIdx, fmt.Sprintf(
				"BEGIN NOT ATOMIC FOR i IN 1..%d DO INSERT INTO %s.%s (v) VALUES (i); END FOR; END",
				backlogPerTurn, raceDB, raceTable,
			))
			Eventually(func(g Gomega) bool {
				primaryClient := sqlClientForTestPod(mdb, primaryIdx)
				defer primaryClient.Close()
				binlogPos, err := primaryClient.GtidBinlogPos(testCtx)
				g.Expect(err).ToNot(HaveOccurred())

				replicaClient := sqlClientForTestPod(mdb, replicaIdx)
				defer replicaClient.Close()
				status, err := replicaClient.ReplicaStatus(testCtx, GinkgoLogr)
				g.Expect(err).ToNot(HaveOccurred())
				return ptr.Deref(status.GtidIOPos, "") == binlogPos
			}, testTimeout, testInterval).Should(BeTrue())

			By(fmt.Sprintf("Round %d: drifting the replica while its SQL thread applies the backlog", round))
			executeSqlInPodByIndex(mdb, replicaIdx, "SET GLOBAL read_only=0;")
			executeSqlInPodByIndex(mdb, replicaIdx, "START SLAVE SQL_THREAD;")
			Eventually(func(g Gomega) bool {
				g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
				patch := client.MergeFrom(mdb.DeepCopy())
				if mdb.Annotations == nil {
					mdb.Annotations = map[string]string{}
				}
				mdb.Annotations[reconcileAnno] = strconv.Itoa(round)
				g.Expect(k8sClient.Patch(testCtx, mdb, patch)).To(Succeed())
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By(fmt.Sprintf("Round %d: expecting the operator to reconfigure the replica without losing its position", round))
			Eventually(func(g Gomega) bool {
				readOnly, err := readOnlyInPodByIndex(mdb, replicaIdx)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(readOnly).To(BeTrue())

				primaryRows, err := countIn(primaryIdx)
				g.Expect(err).ToNot(HaveOccurred())
				replicaRows, err := countIn(replicaIdx)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(replicaRows).To(Equal(primaryRows))

				replicaClient := sqlClientForTestPod(mdb, replicaIdx)
				defer replicaClient.Close()
				status, err := replicaClient.ReplicaStatus(testCtx, GinkgoLogr)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ptr.Deref(status.LastSQLErrno, 0)).To(Equal(0))
				g.Expect(status.IsIOThreadRunning()).To(BeTrue())
				g.Expect(status.IsSQLThreadRunning()).To(BeTrue())
				return true
			}, testHighTimeout, testInterval).Should(BeTrue())
		}
	})
})
