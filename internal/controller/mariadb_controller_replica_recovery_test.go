package controller

import (
	"time"

	"github.com/go-logr/zapr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("isRecoverableError", func() {
	logger := zapr.NewLogger(zap.NewNop())

	DescribeTable("should evaluate recoverability",
		func(buildReplicaStatus func() mariadbv1alpha1.ReplicaStatus, mdb *mariadbv1alpha1.MariaDB, expected bool) {
			res := isRecoverableError(mdb, buildReplicaStatus(), recoverableIOErrorCodes, logger)
			Expect(res).To(Equal(expected))
		},
		Entry("recoverable IO code matches",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1236),
						LastSQLErrno: nil,
					},
					LastErrorTransitionTime: metav1.Time{},
				}
			},
			&mariadbv1alpha1.MariaDB{},
			true,
		),
		Entry("no errors -> not recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  nil,
						LastSQLErrno: nil,
					},
					LastErrorTransitionTime: metav1.Time{},
				}
			},
			&mariadbv1alpha1.MariaDB{},
			false,
		),
		Entry("recent error within threshold -> not recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(0),
					},
					LastErrorTransitionTime: metav1.Now(),
				}
			},
			&mariadbv1alpha1.MariaDB{},
			false,
		),
		Entry("old error older than threshold -> recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(0),
					},
					LastErrorTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
				}
			},
			&mariadbv1alpha1.MariaDB{},
			true,
		),
		Entry("old SQL error older than threshold -> recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(1062),
					},
					LastErrorTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
				}
			},
			&mariadbv1alpha1.MariaDB{},
			true,
		),
		Entry("old SQL error older than custom threshold -> recoverable",
			func() mariadbv1alpha1.ReplicaStatus {
				return mariadbv1alpha1.ReplicaStatus{
					ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
						LastIOErrno:  ptr.To(1),
						LastSQLErrno: ptr.To(1062),
					},
					LastErrorTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
				}
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Replica: mariadbv1alpha1.ReplicaReplication{
								ReplicaRecovery: &mariadbv1alpha1.ReplicaRecovery{
									Enabled:                true,
									ErrorDurationThreshold: &metav1.Duration{Duration: 30 * time.Second},
								},
							},
						},
					},
				},
			},
			true,
		),
	)
})

var _ = Describe("MariaDB Replica Recovery", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	BeforeEach(func() {
		mdb = buildTestMariaDBRecovery(key)
		applyMariadbTestConfig(mdb)

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

	DescribeTable(
		"should recover",
		func(
			backupKey types.NamespacedName,
			builderFn physicalBackupBuilder,
			cleanupFn func(backupKey types.NamespacedName) func(),
		) {
			podIndexToDelete := 2
			backup := builderFn(backupKey)
			backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{
				Suspend: true,
			}
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, backup))).To(Succeed())
			})

			DeferCleanup(func() {
				deletePhysicalBackup(backupKey)
				cleanupFn(backupKey)()
			})

			By("Bootstrapping Recovery")
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
						ErrorDurationThreshold: ptr.To(metav1.Duration{Duration: time.Second * 15}),
					},
				}

				return k8sClient.Update(testCtx, mdb) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Flushing Binary Logs")
			query := `FLUSH LOGS;`
			executeSqlInPodByIndex(mdb, 0, query)
			query = `PURGE BINARY LOGS BEFORE NOW();`
			executeSqlInPodByIndex(mdb, 0, query)

			By("Deleting the First Replica PVC")
			deletePVCByPodIndex(mdb, podIndexToDelete)

			By("Deleting the First Replica Pod")
			deletePodByIndex(mdb, podIndexToDelete)

			By("Flushing Binary Logs")
			query = `FLUSH LOGS;`
			executeSqlInPodByIndex(mdb, 0, query)
			query = `PURGE BINARY LOGS BEFORE NOW();`
			executeSqlInPodByIndex(mdb, 0, query)

			// Otherwise the `pod` doesn't get deleted and gets stuck in `Completed`
			By("Removing PVC finalizer after a short delay")
			time.Sleep(10 * time.Second)

			By("Flushing Binary Logs")
			query = `FLUSH LOGS;`
			executeSqlInPodByIndex(mdb, 0, query)
			query = `PURGE BINARY LOGS BEFORE NOW();`
			executeSqlInPodByIndex(mdb, 0, query)

			var pvc corev1.PersistentVolumeClaim
			pvcKey := mdb.PVCKey(builder.StorageVolume, podIndexToDelete)
			err := k8sClient.Get(testCtx, pvcKey, &pvc)

			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			if err == nil && pvc.DeletionTimestamp != nil {
				Expect(err).NotTo(HaveOccurred())

				pvc.SetFinalizers(nil)
				Expect(k8sClient.Update(testCtx, &pvc)).NotTo(HaveOccurred())
			}

			By("Expecting MariaDB to have recovered eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				return mdb.IsReady() &&
					meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered) &&
					mdb.Status.Replicas == int32(3)

			}, testHighTimeout, testInterval).Should(BeTrue())
		},
		Entry(
			"from physical backup",
			types.NamespacedName{Name: "replication-s3-recovery-test", Namespace: key.Namespace},
			buildPhysicalBackupWithS3Storage(key, "test-replication-recovery", ""),
			func(backupKey types.NamespacedName) func() {
				return func() {
					// No cleanup for s3
				}
			},
		),
		Entry(
			"from volume snapshot",
			types.NamespacedName{Name: "replication-volume-snapshot-recovery-test", Namespace: key.Namespace},
			buildPhysicalBackupWithVolumeSnapshotStorage(key),
			func(backupKey types.NamespacedName) func() {
				return func() {
					By("Deleting Backup Resources")
					opts := []client.DeleteAllOfOption{
						client.MatchingLabels{
							metadata.PhysicalBackupNameLabel: backupKey.Name,
						},
						client.InNamespace(backupKey.Namespace),
					}
					Expect(k8sClient.DeleteAllOf(testCtx, &volumesnapshotv1.VolumeSnapshot{}, opts...)).To(Succeed())
				}
			},
		),
	)
})

func buildTestMariaDBRecovery(key types.NamespacedName) *mariadbv1alpha1.MariaDB {
	return &mariadbv1alpha1.MariaDB{
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
				max_allowed_packet=256M
				general_log`,
			),
			Replication: &mariadbv1alpha1.Replication{
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex:          ptr.To(0),
						AutomaticFailover: ptr.To(true),
					},
					Replica: mariadbv1alpha1.ReplicaReplication{},
				},
				Enabled: true,
			},
			Replicas: 3,
			Storage: mariadbv1alpha1.Storage{
				Size:                ptr.To(resource.MustParse("300Mi")),
				StorageClassName:    "csi-hostpath-sc",
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
}
