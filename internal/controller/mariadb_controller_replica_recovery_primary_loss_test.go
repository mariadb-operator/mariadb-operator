package controller

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A replica recovery removes the StatefulSet (leaving the Pods orphaned) so the replica under recovery can
// be rebuilt. If the primary Pod dies in that window, nothing recreates it: the recovery backup keeps
// waiting for a primary that no longer exists and the cluster stays down. The operator must bring the
// StatefulSet back so the primary returns and the recovery completes.
var _ = Describe("MariaDB replica recovery primary loss", Ordered, func() {
	var (
		mdbKey    = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		backupKey = types.NamespacedName{Name: "mariadb-repl-primary-loss-recovery", Namespace: testNamespace}
		mdb       *mariadbv1alpha1.MariaDB
	)

	const (
		markerDB    = "primarylossdb"
		markerTable = "marker"
	)

	BeforeAll(func() {
		backup := buildPhysicalBackupWithS3Storage(mdbKey, "test-replication-recovery", "primary-loss")(backupKey)
		backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{Suspend: true}
		backup.Spec.Target = ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica)
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			deletePhysicalBackup(backupKey)
		})

		mdb = buildTestMariaDBRecovery(mdbKey)
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
			deleteMariadb(mdbKey, true)
		})
		Eventually(func() bool {
			return k8sClient.Get(testCtx, mdbKey, mdb) == nil && mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		executeSqlInPodByIndex(mdb, 0, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", markerDB))
		executeSqlInPodByIndex(mdb, 0, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY);", markerDB, markerTable))
		executeSqlInPodByIndex(mdb, 0, fmt.Sprintf("INSERT INTO %s.%s VALUES (1);", markerDB, markerTable))
		Eventually(func() bool {
			exists, err := tableExistsInPodByIndex(mdb, 1, markerDB, markerTable)
			return err == nil && exists
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("recreates the StatefulSet when the primary Pod disappears during a PVC recovery", func() {
		stsKey := client.ObjectKeyFromObject(mdb)
		primaryKey := types.NamespacedName{Name: "mariadb-repl-0", Namespace: testNamespace}

		By("Diverging the replica so replication fails and a recovery starts")
		executeSqlInPodByIndex(mdb, 1, "SET GLOBAL read_only=OFF;")
		executeSqlInPodByIndex(mdb, 1, fmt.Sprintf("INSERT INTO %s.%s VALUES (2);", markerDB, markerTable))
		executeSqlInPodByIndex(mdb, 1, "SET GLOBAL read_only=ON;")
		executeSqlInPodByIndex(mdb, 0, fmt.Sprintf("INSERT INTO %s.%s VALUES (2);", markerDB, markerTable))

		By("Expecting the recovery to remove the StatefulSet, then losing the primary Pod in that window")
		backupCompleteBeforeLoss := false
		Eventually(func() bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(testCtx, stsKey, &sts); !apierrors.IsNotFound(err) {
				return false
			}
			var backup mariadbv1alpha1.PhysicalBackup
			if err := k8sClient.Get(testCtx, mdb.PhysicalBackupReplicaRecoveryKey(), &backup); err == nil {
				backupCompleteBeforeLoss = meta.IsStatusConditionTrue(backup.Status.Conditions, mariadbv1alpha1.ConditionTypeComplete)
			}
			var pod corev1.Pod
			if err := k8sClient.Get(testCtx, primaryKey, &pod); err != nil {
				return apierrors.IsNotFound(err)
			}
			return k8sClient.Delete(testCtx, &pod, &client.DeleteOptions{
				GracePeriodSeconds: ptr.To(int64(0)),
			}) == nil
		}, testHighTimeout, 200*time.Millisecond).Should(BeTrue())
		GinkgoWriter.Printf("primary Pod deleted while the StatefulSet was absent; recovery backup already complete: %v\n",
			backupCompleteBeforeLoss)

		By("Expecting the StatefulSet to be recreated so the primary returns")
		Eventually(func() bool {
			var sts appsv1.StatefulSet
			return k8sClient.Get(testCtx, stsKey, &sts) == nil
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting the replica recovery to complete")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
				return false
			}
			return mdb.IsReady() &&
				meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered) &&
				mdb.Status.Replicas == int32(2)
		}, 3*testHighTimeout, testInterval).Should(BeTrue())

		By("Verifying the data survived on both Pods")
		for _, idx := range []int{0, 1} {
			Eventually(func() bool {
				exists, err := tableExistsInPodByIndex(mdb, idx, markerDB, markerTable)
				return err == nil && exists
			}, testTimeout, testInterval).Should(BeTrue(), "marker table missing on mariadb-repl-%d", idx)
		}
	})
})
