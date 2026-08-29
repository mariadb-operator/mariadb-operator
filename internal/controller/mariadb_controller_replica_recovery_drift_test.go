package controller

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// This suite reproduces a deadlock between the replica recovery phase and the replication phase.
// Replica recovery requeues until its PhysicalBackup completes, returning from the reconcile loop
// before replication is reconciled. A primary replicating from the very Pod under recovery therefore
// stays unready forever, which in turn leaves the recovery PhysicalBackup without a target Pod:
// the replica is being rebuilt and the primary is not ready. Neither side can make progress.
var _ = Describe("MariaDB Replica Recovery Primary Drift", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		backupKey = types.NamespacedName{
			Name:      "mariadb-repl-drift-recovery",
			Namespace: testNamespace,
		}
		mdb          *mariadbv1alpha1.MariaDB
		replicaIndex = 1
		phantomIndex = 9
	)

	const (
		driftDatabase = "driftdb"
		driftTable    = "drift"
		driftRowID    = 1
	)

	BeforeAll(func() {
		mdb = buildTestMariaDBPrimaryDrift(key)
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
		}, testVeryHighTimeout, testInterval).Should(BeTrue())
	})

	It("should recover a replica while the primary replicates from the recovered Pod", func() {
		backup := buildPhysicalBackupWithS3Storage(key, "test-replication-recovery", "primary-drift")(backupKey)
		backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{
			Suspend: true,
		}
		backup.Spec.Target = ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica)
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			deletePhysicalBackup(backupKey)
		})

		By("Bootstrapping recovery")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Replication.Replica = mariadbv1alpha1.ReplicaReplication{
				ReplicaBootstrapFrom: &mariadbv1alpha1.ReplicaBootstrapFrom{
					PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{
						Name: backupKey.Name,
					},
				},
				ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
					Enabled:                true,
					ErrorDurationThreshold: ptr.To(metav1.Duration{Duration: 15 * time.Second}),
				},
			}
			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		Expect(k8sClient.Get(testCtx, key, mdb)).To(Succeed())
		primaryIndex := ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, 0)

		By("Creating a table on the primary")
		executeSqlInPodByIndex(mdb, primaryIndex, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", driftDatabase))
		executeSqlInPodByIndex(mdb, primaryIndex, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY);", driftDatabase, driftTable,
		))

		By("Expecting the replica to have replicated the table")
		Eventually(func() bool {
			exists, err := tableExistsInPodByIndex(mdb, replicaIndex, driftDatabase, driftTable)
			return err == nil && exists
		}, testTimeout, testInterval).Should(BeTrue())

		By("Diverging the replica dataset from the primary")
		executeSqlInPodByIndex(mdb, replicaIndex, "SET GLOBAL read_only=OFF;")
		executeSqlInPodByIndex(mdb, replicaIndex, fmt.Sprintf(
			"INSERT INTO %s.%s VALUES (%d);", driftDatabase, driftTable, driftRowID,
		))
		executeSqlInPodByIndex(mdb, replicaIndex, "SET GLOBAL read_only=ON;")
		executeSqlInPodByIndex(mdb, primaryIndex, fmt.Sprintf(
			"INSERT INTO %s.%s VALUES (%d);", driftDatabase, driftTable, driftRowID,
		))

		By("Expecting replica recovery to be in flight")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsRecoveringReplicas()
		}, testHighTimeout, testInterval).Should(BeTrue())

		changeMaster := fmt.Sprintf(
			"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='repl', MASTER_USE_GTID=current_pos;",
			stsobj.PodFQDNWithService(mdb.ObjectMeta, phantomIndex, mdb.InternalServiceKey().Name),
			mdb.Spec.Port,
		)

		By("Pointing the primary at the Pod being recovered until it becomes unready")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			if !mdb.IsRecoveringReplicas() {
				return false
			}
			isReplica, err := replicaInPodByIndex(mdb, primaryIndex)
			if err != nil {
				return false
			}
			if !isReplica {
				if err := execSqlInPodByIndex(mdb, primaryIndex, changeMaster); err != nil {
					return false
				}
				if err := execSqlInPodByIndex(mdb, primaryIndex, "START SLAVE;"); err != nil {
					return false
				}
				return false
			}
			ready, err := podReadyByIndex(mdb, primaryIndex)
			return err == nil && !ready
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting the primary to stop replicating from the missing Pod")
		Eventually(func() bool {
			isReplica, err := replicaInPodByIndex(mdb, primaryIndex)
			return err == nil && !isReplica
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to have recovered eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() &&
				meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered) &&
				mdb.Status.Replicas == int32(2)
		}, testVeryHighTimeout, testInterval).Should(BeTrue())
	})
})

func buildTestMariaDBPrimaryDrift(key types.NamespacedName) *mariadbv1alpha1.MariaDB {
	mdb := &mariadbv1alpha1.MariaDB{
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
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex:     ptr.To(0),
						AutoFailover: ptr.To(false),
					},
					Replica: mariadbv1alpha1.ReplicaReplication{},
				},
				Enabled: true,
			},
			Replicas: 2,
			Storage: mariadbv1alpha1.Storage{
				Size:                ptr.To(resource.MustParse("300Mi")),
				StorageClassName:    "csi-hostpath-sc",
				ResizeInUseVolumes:  ptr.To(true),
				WaitForVolumeResize: ptr.To(true),
			},
			Service: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.120",
					},
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
			SecondaryService: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.131",
					},
				},
			},
			UpdateStrategy: mariadbv1alpha1.UpdateStrategy{
				Type: mariadbv1alpha1.ReplicasFirstPrimaryLastUpdateType,
			},
		},
	}
	// A fast readiness probe keeps the wedged primary observable: the deadlock only starts once
	// the drifted primary is reported unready.
	mdb.Spec.ReadinessProbe = &mariadbv1alpha1.Probe{
		InitialDelaySeconds: 10,
		TimeoutSeconds:      2,
		PeriodSeconds:       2,
		FailureThreshold:    1,
	}
	return mdb
}
