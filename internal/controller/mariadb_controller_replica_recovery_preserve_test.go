package controller

import (
	"fmt"
	"strconv"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A replica in error is wiped and rebuilt by the recovery. It may be a demoted primary holding the only copy of
// its last transactions, so its storage is snapshotted first. A SQL thread stopped on purpose, as
// mariadb-backup --safe-slave-backup does during a physical backup of the replica, is neither a reason to
// restart the container nor a stall to recover from.
var _ = Describe("MariaDB replica recovery preserve", Ordered, func() {
	var (
		key       = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		backupKey = types.NamespacedName{Name: "replication-preserve-recovery-test", Namespace: testNamespace}
		mdb       *mariadbv1alpha1.MariaDB
	)

	const (
		preserveDB        = "preservedb"
		preserveTable     = "orders"
		replicaIdx        = 1
		primaryIdx        = 0
		conflictingID     = 1000
		sqlStoppedWindow  = 70 * time.Second
		snapshotClassName = "csi-hostpath-snapclass"
	)

	rowCountInPod := func(podIndex int) int {
		client := sqlClientForTestPod(mdb, podIndex)
		defer client.Close()
		row, err := client.QueryColumnMap(testCtx, fmt.Sprintf("SELECT COUNT(*) AS c FROM %s.%s;", preserveDB, preserveTable))
		Expect(err).ToNot(HaveOccurred())
		c, err := strconv.Atoi(row["c"])
		Expect(err).ToNot(HaveOccurred())
		return c
	}

	mariadbContainerRestarts := func(podIndex int) int32 {
		var pod corev1.Pod
		Expect(k8sClient.Get(testCtx, types.NamespacedName{
			Name:      stsobj.PodName(mdb.ObjectMeta, podIndex),
			Namespace: mdb.Namespace,
		}, &pod)).To(Succeed())
		var restarts int32
		for _, status := range pod.Status.ContainerStatuses {
			restarts += status.RestartCount
		}
		return restarts
	}

	BeforeAll(func() {
		backup := buildPhysicalBackupWithS3Storage(key, "test-replication-recovery", "preserve")(backupKey)
		backup.Spec.Target = ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica)
		backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{Suspend: true}
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, backup))).To(Succeed())
			deletePhysicalBackup(backupKey)
		})

		mdb = buildTestMariaDBRecovery(key)
		mdb.Spec.Replicas = 2
		mdb.Spec.Replication.Replica = mariadbv1alpha1.ReplicaReplication{
			ReplicaBootstrapFrom: &mariadbv1alpha1.ReplicaBootstrapFrom{
				PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{Name: backupKey.Name},
			},
			ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
				Enabled:                         true,
				ErrorDurationThreshold:          ptr.To(metav1.Duration{Duration: 15 * time.Second}),
				PreserveVolumeSnapshotClassName: ptr.To(snapshotClassName),
			},
		}
		applyMariadbTestConfig(mdb)
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(key, true)
			Expect(k8sClient.DeleteAllOf(testCtx, &volumesnapshotv1.VolumeSnapshot{},
				client.InNamespace(key.Namespace),
				client.MatchingLabels{replicaRecoveryPreserveLabel: key.Name},
			)).To(Succeed())
		})
		Eventually(func() bool {
			return k8sClient.Get(testCtx, key, mdb) == nil && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		executeSqlInPodByIndex(mdb, primaryIdx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", preserveDB))
		executeSqlInPodByIndex(mdb, primaryIdx,
			fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY, v VARCHAR(32));", preserveDB, preserveTable))
		executeSqlInPodByIndex(mdb, primaryIdx,
			fmt.Sprintf("INSERT INTO %s.%s (id, v) VALUES (1,'primary'),(2,'primary');", preserveDB, preserveTable))
		Eventually(func() bool {
			exists, err := tableExistsInPodByIndex(mdb, replicaIdx, preserveDB, preserveTable)
			return err == nil && exists
		}, testHighTimeout, testInterval).Should(BeTrue())
	})

	It("keeps a replica whose SQL thread was stopped without an error", func() {
		restartsBefore := mariadbContainerRestarts(replicaIdx)

		By("Stopping the replica SQL thread, as mariadb-backup --safe-slave-backup does")
		executeSqlInPodByIndex(mdb, replicaIdx, "STOP SLAVE SQL_THREAD;")
		DeferCleanup(func() {
			Expect(execSqlInPodByIndex(mdb, replicaIdx, "START SLAVE SQL_THREAD;")).To(Succeed())
		})

		By("Expecting neither a container restart nor a recovery while it stays stopped")
		Consistently(func(g Gomega) bool {
			g.Expect(mariadbContainerRestarts(replicaIdx)).To(Equal(restartsBefore))
			g.Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
			g.Expect(getActiveReplicaRecoveryReplicas(mdb)).To(BeEmpty())
			return true
		}, sqlStoppedWindow, testInterval).Should(BeTrue())
	})

	It("snapshots an errored replica before rebuilding it", func() {
		Expect(execSqlInPodByIndex(mdb, replicaIdx, "START SLAVE SQL_THREAD;")).To(Succeed())
		replicaPVC := mdb.PVCKey(builder.StorageVolume, replicaIdx)

		By("Writing a row only the replica holds, then the conflicting row on the primary")
		executeSqlInPodByIndex(mdb, replicaIdx,
			fmt.Sprintf("INSERT INTO %s.%s (id, v) VALUES (%d,'replica-only');", preserveDB, preserveTable, conflictingID))
		executeSqlInPodByIndex(mdb, primaryIdx,
			fmt.Sprintf("INSERT INTO %s.%s (id, v) VALUES (%d,'primary');", preserveDB, preserveTable, conflictingID))

		By("Expecting a ready preserve snapshot of the replica storage")
		Eventually(func(g Gomega) bool {
			var snapshots volumesnapshotv1.VolumeSnapshotList
			g.Expect(k8sClient.List(testCtx, &snapshots,
				client.InNamespace(key.Namespace),
				client.MatchingLabels{
					replicaRecoveryPreserveLabel:         key.Name,
					replicaRecoveryPreservePodIndexLabel: strconv.Itoa(replicaIdx),
				},
			)).To(Succeed())
			g.Expect(snapshots.Items).To(HaveLen(1))
			snapshot := snapshots.Items[0]
			g.Expect(ptr.Deref(snapshot.Spec.Source.PersistentVolumeClaimName, "")).To(Equal(replicaPVC.Name))
			g.Expect(ptr.Deref(snapshot.Spec.VolumeSnapshotClassName, "")).To(Equal(snapshotClassName))
			g.Expect(ptr.Deref(ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{}).ReadyToUse, false)).To(BeTrue())
			return true
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting the replica to be rebuilt from the primary")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() &&
				meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered) &&
				len(getActiveReplicaRecoveryReplicas(mdb)) == 0
		}, testHighTimeout, testInterval).Should(BeTrue())
		Eventually(func() int {
			return rowCountInPod(replicaIdx)
		}, testHighTimeout, testInterval).Should(Equal(rowCountInPod(primaryIdx)))
	})
})
