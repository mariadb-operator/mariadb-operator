package controller

import (
	"fmt"
	"strconv"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A replica whose data drifted without a transaction of its own holds nothing the primary lacks, so it is recovered
// without a preserve VolumeSnapshotClass. 26.3.8 checked that only after the recovery had stopped the replica Pod,
// could never reach the replica, and refused every such recovery.
var _ = Describe("MariaDB replica recovery without a preserve VolumeSnapshotClass", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		backupKey = types.NamespacedName{
			Name:      "physicalbackup-clean-recovery",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	const (
		markerDB    = "cleanrecoverydb"
		markerTable = "marker"
		primaryIdx  = 0
		replicaIdx  = 1
	)

	countIn := func(podIndex int) (int, error) {
		sqlClient := sqlClientForTestPod(mdb, podIndex)
		defer sqlClient.Close()
		row, err := sqlClient.QueryColumnMap(testCtx, fmt.Sprintf("SELECT COUNT(*) AS c FROM %s.%s;", markerDB, markerTable))
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(row["c"])
	}

	BeforeAll(func() {
		backup := buildPhysicalBackupWithS3Storage(key, "test-replication-recovery", "clean")(backupKey)
		backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{Suspend: true}
		backup.Spec.Target = ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica)
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			deletePhysicalBackup(backupKey)
		})

		mdb = buildTestMariaDBRecovery(key)
		mdb.Spec.Replicas = 2
		mdb.Spec.Replication.Replica = mariadbv1alpha1.ReplicaReplication{
			ReplicaBootstrapFrom: &mariadbv1alpha1.ReplicaBootstrapFrom{
				PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{Name: backupKey.Name},
			},
			ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
				Enabled:                true,
				ErrorDurationThreshold: ptr.To(metav1.Duration{Duration: 15 * time.Second}),
			},
		}
		applyMariadbTestConfig(mdb)
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, true)
		})
		Eventually(func() bool {
			return k8sClient.Get(testCtx, key, mdb) == nil && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		executeSqlInPodByIndex(mdb, primaryIdx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", markerDB))
		executeSqlInPodByIndex(mdb, primaryIdx,
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY, v INT);", markerDB, markerTable))
		executeSqlInPodByIndex(mdb, primaryIdx, fmt.Sprintf("INSERT INTO %s.%s VALUES (1, 1), (2, 1);", markerDB, markerTable))
		Eventually(func() bool {
			rows, err := countIn(replicaIdx)
			return err == nil && rows == 2
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("recovers a replica that holds no transactions of its own", func() {
		By("Drifting the replica data without writing a transaction to its binary log")
		executeSqlInPodByIndex(mdb, replicaIdx,
			fmt.Sprintf("SET STATEMENT sql_log_bin=0 FOR DELETE FROM %s.%s WHERE id = 1;", markerDB, markerTable))
		executeSqlInPodByIndex(mdb, primaryIdx, fmt.Sprintf("UPDATE %s.%s SET v = 2 WHERE id = 1;", markerDB, markerTable))

		By("Expecting the replica recovery to start")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsRecoveringReplicas()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting the replica recovery to complete without a preserve snapshot")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() &&
				meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered) &&
				mdb.Status.Replicas == int32(2)
		}, testVeryHighTimeout, testInterval).Should(BeTrue())

		var snapshots volumesnapshotv1.VolumeSnapshotList
		Expect(k8sClient.List(testCtx, &snapshots,
			client.InNamespace(key.Namespace),
			client.MatchingLabels{replicaRecoveryPreserveLabel: key.Name},
		)).To(Succeed())
		Expect(snapshots.Items).To(BeEmpty())

		By("Expecting the recovered replica to hold the primary data")
		Eventually(func(g Gomega) bool {
			primaryRows, err := countIn(primaryIdx)
			g.Expect(err).ToNot(HaveOccurred())
			replicaRows, err := countIn(replicaIdx)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(replicaRows).To(Equal(primaryRows))
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	})
})
