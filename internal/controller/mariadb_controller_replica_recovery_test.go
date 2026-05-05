package controller

import (
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("isRecoverableError", func() {
	logger := logr.Discard()

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

// getReplicasToRecover gates on isRecoverableError, which only returns true
// for errors whose age exceeds errorDurationThreshold (default 5m). When
// mariadb is crashlooping mid-recovery, every container restart refreshes
// LastErrorTransitionTime and the replica drops out of the recoverable list
// for a window. The outer reconcile interprets the empty list as "all
// healthy", calls setReplicaRecoveredAndCleanup, and tears down the in-flight
// pb-recovery PB and pb-init Job.  These tests pin the post-fix behavior
// where an in-flight recovery keeps the replica in the list as long as it
// still reports any replication error.
var _ = Describe("getReplicasToRecover", func() {
	logger := logr.Discard()
	freshError := metav1.Now()
	emptyMdb := &mariadbv1alpha1.MariaDB{}
	recoveringMdb := func() *mariadbv1alpha1.MariaDB {
		mdb := &mariadbv1alpha1.MariaDB{}
		meta.SetStatusCondition(&mdb.Status.Conditions, metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeReplicaRecovered,
			Status:  metav1.ConditionFalse,
			Reason:  mariadbv1alpha1.ConditionReasonReplicaRecovering,
			Message: "Recovering replica",
		})
		return mdb
	}
	withReplicas := func(mdb *mariadbv1alpha1.MariaDB, replicas map[string]mariadbv1alpha1.ReplicaStatus) *mariadbv1alpha1.MariaDB {
		mdb.Status.Replication = &mariadbv1alpha1.ReplicationStatus{
			Replicas: replicas,
		}
		return mdb
	}

	It("returns empty when no replicas exist", func() {
		Expect(getReplicasToRecover(emptyMdb, logger)).To(BeEmpty())
	})

	It("returns replicas with recoverable IO codes regardless of recovering state", func() {
		mdb := withReplicas(emptyMdb.DeepCopy(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r0": {
				ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
					LastIOErrno: ptr.To(1236),
				},
			},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(ConsistOf("r0"))
	})

	It("returns replicas with errors past the threshold regardless of recovering state", func() {
		old := metav1.NewTime(time.Now().Add(-10 * time.Minute))
		mdb := withReplicas(emptyMdb.DeepCopy(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r0": {
				ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
					LastSQLErrno: ptr.To(1032),
				},
				LastErrorTransitionTime: old,
			},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(ConsistOf("r0"))
	})

	It("excludes replicas with fresh errors when not recovering", func() {
		mdb := withReplicas(emptyMdb.DeepCopy(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r0": {
				ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
					LastSQLErrno: ptr.To(1032),
				},
				LastErrorTransitionTime: freshError,
			},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(BeEmpty())
	})

	It("includes replicas with fresh errors when already recovering (regression guard for medicine-stg2 oscillation)", func() {
		mdb := withReplicas(recoveringMdb(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r0": {
				ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
					LastSQLErrno: ptr.To(1032),
				},
				LastErrorTransitionTime: freshError,
			},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(ConsistOf("r0"))
	})

	It("includes replicas with fresh IO errors when already recovering", func() {
		mdb := withReplicas(recoveringMdb(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r0": {
				ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
					LastIOErrno: ptr.To(1045),
				},
				LastErrorTransitionTime: freshError,
			},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(ConsistOf("r0"))
	})

	It("excludes replicas with no error even while recovering (genuine recovery completion path)", func() {
		mdb := withReplicas(recoveringMdb(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r0": {
				ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{
					LastIOErrno:  ptr.To(0),
					LastSQLErrno: ptr.To(0),
				},
			},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(BeEmpty())
	})

	It("returns replicas in deterministic sorted order", func() {
		mdb := withReplicas(recoveringMdb(), map[string]mariadbv1alpha1.ReplicaStatus{
			"r2": {ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{LastIOErrno: ptr.To(1045)}, LastErrorTransitionTime: freshError},
			"r0": {ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{LastIOErrno: ptr.To(1045)}, LastErrorTransitionTime: freshError},
			"r1": {ReplicaStatusVars: mariadbv1alpha1.ReplicaStatusVars{LastIOErrno: ptr.To(1045)}, LastErrorTransitionTime: freshError},
		})
		Expect(getReplicasToRecover(mdb, logger)).To(Equal([]string{"r0", "r1", "r2"}))
	})
})

var _ = Describe("MariaDB Replica Recovery", Ordered, func() {
	var (
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	var primaryPodIndex int

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

		primaryPodIndex = ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, 0)
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

			By("Deleting the First Replica PVC")
			deletePVCByPodIndex(mdb, podIndexToDelete)

			By("Deleting the First Replica Pod")
			deletePodByIndex(mdb, podIndexToDelete)

			By("Flushing Binary Logs Continuously Until Replica Recovery is needed")
			Eventually(func() bool {
				query := `FLUSH LOGS;`
				executeSqlInPodByIndex(mdb, primaryPodIndex, query)
				query = `PURGE BINARY LOGS BEFORE NOW();`
				executeSqlInPodByIndex(mdb, primaryPodIndex, query)

				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}

				// Adding mariadbv1alpha1.ConditionTypeReplicaRecovered just in case, should never be true, but we don't want to get stuck
				return meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered) ||
					meta.IsStatusConditionFalse(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered)
			}, testTimeout, time.Second*2).Should(BeTrue())

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
						PodIndex:     ptr.To(0),
						AutoFailover: ptr.To(true),
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

var _ = Describe("replicaRecoveryVerifier", func() {
	healthy := func() *mariadbv1alpha1.ReplicaStatusVars {
		return &mariadbv1alpha1.ReplicaStatusVars{
			LastIOErrno:  ptr.To(0),
			LastSQLErrno: ptr.To(0),
		}
	}
	withSQLError := func() *mariadbv1alpha1.ReplicaStatusVars {
		return &mariadbv1alpha1.ReplicaStatusVars{
			LastIOErrno:  ptr.To(0),
			LastSQLErrno: ptr.To(1032),
		}
	}
	withIOError := func() *mariadbv1alpha1.ReplicaStatusVars {
		return &mariadbv1alpha1.ReplicaStatusVars{
			LastIOErrno:  ptr.To(1236),
			LastSQLErrno: ptr.To(0),
		}
	}

	It("does not declare recovery on the first healthy observation", func() {
		clock := time.Unix(0, 0)
		v := newReplicaRecoveryVerifier(30 * time.Second)
		v.now = func() time.Time { return clock }

		stable, reason := v.observe(healthy())
		Expect(stable).To(BeFalse())
		Expect(reason).To(ContainSubstring("verification window"))
	})

	It("declares recovery only after the verification window elapses", func() {
		clock := time.Unix(0, 0)
		v := newReplicaRecoveryVerifier(30 * time.Second)
		v.now = func() time.Time { return clock }

		stable, _ := v.observe(healthy())
		Expect(stable).To(BeFalse())

		clock = clock.Add(29 * time.Second)
		stable, _ = v.observe(healthy())
		Expect(stable).To(BeFalse())

		clock = clock.Add(2 * time.Second)
		stable, _ = v.observe(healthy())
		Expect(stable).To(BeTrue())
	})

	It("resets the timer when an SQL error appears mid-window", func() {
		clock := time.Unix(0, 0)
		v := newReplicaRecoveryVerifier(30 * time.Second)
		v.now = func() time.Time { return clock }

		_, _ = v.observe(healthy())

		clock = clock.Add(20 * time.Second)
		stable, reason := v.observe(withSQLError())
		Expect(stable).To(BeFalse())
		Expect(reason).To(ContainSubstring("replication error"))

		clock = clock.Add(29 * time.Second)
		stable, _ = v.observe(healthy())
		Expect(stable).To(BeFalse(), "timer must restart from the error observation, not accumulate")

		clock = clock.Add(31 * time.Second)
		stable, _ = v.observe(healthy())
		Expect(stable).To(BeTrue())
	})

	It("treats IO errors the same as SQL errors", func() {
		clock := time.Unix(0, 0)
		v := newReplicaRecoveryVerifier(30 * time.Second)
		v.now = func() time.Time { return clock }

		_, _ = v.observe(healthy())
		clock = clock.Add(15 * time.Second)
		stable, _ := v.observe(withIOError())
		Expect(stable).To(BeFalse())

		clock = clock.Add(31 * time.Second)
		stable, _ = v.observe(healthy())
		Expect(stable).To(BeFalse(), "verifier must restart after IO error, not accept the prior healthy run")
	})

	It("treats nil status as unhealthy and resets the timer", func() {
		clock := time.Unix(0, 0)
		v := newReplicaRecoveryVerifier(30 * time.Second)
		v.now = func() time.Time { return clock }

		_, _ = v.observe(healthy())
		clock = clock.Add(20 * time.Second)
		stable, reason := v.observe(nil)
		Expect(stable).To(BeFalse())
		Expect(reason).To(ContainSubstring("status unavailable"))

		clock = clock.Add(31 * time.Second)
		stable, _ = v.observe(healthy())
		Expect(stable).To(BeFalse())
	})

	It("treats unset errno fields as unhealthy", func() {
		clock := time.Unix(0, 0)
		v := newReplicaRecoveryVerifier(30 * time.Second)
		v.now = func() time.Time { return clock }

		stable, _ := v.observe(&mariadbv1alpha1.ReplicaStatusVars{})
		Expect(stable).To(BeFalse())
	})
})
