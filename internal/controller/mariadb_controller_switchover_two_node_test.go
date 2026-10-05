package controller

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// A switchover promotes the candidate before it turns the old primary into a replica. In a two-node cluster
// there is no configured replica left in between, so a switchover that had to be retried after the promotion
// was gated on "any configured replica" and never resumed: the old primary stayed read-locked and read-only,
// the promoted candidate accepted writes the status never acknowledged, and the cluster stayed in
// "Switching primary" until a human reset the spec.
var _ = Describe("MariaDB two-node switchover", Ordered, func() {
	var (
		key = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		mdb *mariadbv1alpha1.MariaDB
	)

	const (
		markerDB       = "switchoverdb"
		markerTable    = "before_switchover"
		switchedTable  = "after_switchover"
		strandedTable  = "during_stranded_switchover"
		suspendSettle  = 5 * time.Second
		oldPrimaryIdx  = 0
		newPrimaryIdx  = 1
		strandedTarget = 0
	)

	updateMariaDB := func(mutate func(mdb *mariadbv1alpha1.MariaDB)) {
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			mutate(mdb)
			g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	}

	expectSwitchedTo := func(podIndex int) {
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() && !mdb.IsSwitchingPrimary() &&
				ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, -1) == podIndex
		}, testHighTimeout, testInterval).Should(BeTrue())
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, podIndex)
			return err == nil && !readOnly
		}, testTimeout, testInterval).Should(BeTrue())
		Eventually(func() bool {
			readOnly, err := readOnlyInPodByIndex(mdb, 1-podIndex)
			return err == nil && readOnly
		}, testTimeout, testInterval).Should(BeTrue())
	}

	expectTableReplicatedTo := func(podIndex int, table string) {
		Eventually(func() bool {
			exists, err := tableExistsInPodByIndex(mdb, podIndex, markerDB, table)
			return err == nil && exists
		}, testTimeout, testInterval).Should(BeTrue(), "table %s.%s missing on Pod %d", markerDB, table, podIndex)
	}

	BeforeAll(func() {
		mdb = buildTestMariaDBWithRepl(key)
		mdb.Spec.Replicas = 2
		applyMariadbTestConfig(mdb)
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, true)
		})
		Eventually(func() bool {
			return k8sClient.Get(testCtx, key, mdb) == nil && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())
	})

	It("promotes the only replica and demotes the old primary", func() {
		By("Writing a marker on the primary and expecting it on the replica")
		executeSqlInPodByIndex(mdb, oldPrimaryIdx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", markerDB))
		executeSqlInPodByIndex(mdb, oldPrimaryIdx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY);", markerDB, markerTable))
		expectTableReplicatedTo(newPrimaryIdx, markerTable)

		By("Promoting the replica")
		updateMariaDB(func(mdb *mariadbv1alpha1.MariaDB) {
			mdb.Spec.Replication.Primary.PodIndex = ptr.To(newPrimaryIdx)
		})

		By("Expecting the switchover to complete")
		expectSwitchedTo(newPrimaryIdx)

		By("Writing on the new primary and expecting it on the old primary")
		executeSqlInPodByIndex(mdb, newPrimaryIdx, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY);", markerDB, switchedTable))
		expectTableReplicatedTo(oldPrimaryIdx, switchedTable)
	})

	It("resumes a switchover stranded after promoting the only replica", func() {
		By("Suspending reconciliation to stage the stranded state")
		updateMariaDB(func(mdb *mariadbv1alpha1.MariaDB) {
			mdb.Spec.Suspend = true
		})
		time.Sleep(suspendSettle)

		By("Marking the switchover as in progress, as the operator does before running its phases")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			condition.SetPrimarySwitching(&mdb.Status, stsobj.PodName(mdb.ObjectMeta, strandedTarget))
			g.Expect(k8sClient.Status().Update(testCtx, mdb)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Promoting the replica by hand, as the 'Configure new primary' phase does, and leaving the primary read-only")
		promoteReplicaByHand(mdb, strandedTarget)
		executeSqlInPodByIndex(mdb, newPrimaryIdx, "SET GLOBAL read_only=1;")

		By("Writing on the promoted Pod while the switchover is stranded")
		executeSqlInPodByIndex(mdb, strandedTarget,
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY);", markerDB, strandedTable))

		By("Requesting the switchover and resuming reconciliation")
		updateMariaDB(func(mdb *mariadbv1alpha1.MariaDB) {
			mdb.Spec.Replication.Primary.PodIndex = ptr.To(strandedTarget)
			mdb.Spec.Suspend = false
		})

		By("Expecting the stranded switchover to complete")
		expectSwitchedTo(strandedTarget)

		By("Expecting the demoted primary to replay the write accepted during the stall")
		expectTableReplicatedTo(newPrimaryIdx, strandedTable)
	})
})

func promoteReplicaByHand(mdb *mariadbv1alpha1.MariaDB, podIndex int) {
	clientSet := sql.NewClientSet(mdb, refresolver.New(k8sClient))
	client, err := clientSet.ClientForIndex(testCtx, podIndex)
	Expect(err).ToNot(HaveOccurred())
	defer client.Close()

	Expect(client.StopAllSlaves(testCtx)).To(Succeed())
	Expect(client.ResetAllSlaves(testCtx)).To(Succeed())
	Expect(client.AlignGtidStateOnPromotion(testCtx)).To(Succeed())
	Expect(client.DisableReadOnly(testCtx)).To(Succeed())
}
