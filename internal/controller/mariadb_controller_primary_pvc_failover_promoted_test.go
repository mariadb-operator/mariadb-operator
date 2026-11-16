package controller

import (
	"strings"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// MaxScale auto_failover can promote the replica before the operator notices that the primary lost its
// storage. The promoted replica is writable and no longer replicates, and it is the only node holding the data.
// The operator keeps the MariaDB suspended until MaxScale has promoted it, which makes that order certain.
var _ = Describe("MariaDB primary PVC failover after MaxScale promotion", Ordered, func() {
	var (
		mdbKey    = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		mxsKey    = types.NamespacedName{Name: "maxscale-repl", Namespace: testNamespace}
		backupKey = types.NamespacedName{Name: "mariadb-repl-pvc-failover-promoted", Namespace: testNamespace}
		mdb       *mariadbv1alpha1.MariaDB
	)

	const promotedPod = "mariadb-repl-1"

	maxScaleState := func(server string) string {
		var m mariadbv1alpha1.MaxScale
		if err := k8sClient.Get(testCtx, mxsKey, &m); err != nil {
			return ""
		}
		for _, s := range m.Status.Servers {
			if s.Name == server {
				return s.State
			}
		}
		return ""
	}
	setSuspend := func(suspend bool) {
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
				return false
			}
			mdb.Spec.Suspend = suspend
			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())
	}

	BeforeAll(func() {
		backup := buildPhysicalBackupWithS3Storage(mdbKey, "test-replication-recovery", "pvc-failover-promoted")(backupKey)
		backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{Suspend: true}
		backup.Spec.Target = ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica)
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			deletePhysicalBackup(backupKey)
		})

		mdb = buildTestMariaDBRecovery(mdbKey)
		mdb.Spec.Replicas = 2
		mdb.Spec.Replication.Primary.AutoFailover = ptr.To(false)
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

		mxs := &mariadbv1alpha1.MaxScale{
			ObjectMeta: metav1.ObjectMeta{Name: mxsKey.Name, Namespace: mxsKey.Namespace},
			Spec: mariadbv1alpha1.MaxScaleSpec{
				Replicas: 1,
				MariaDBRef: &mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{Name: mdbKey.Name, Namespace: mdbKey.Namespace},
				},
				KubernetesService: &mariadbv1alpha1.ServiceTemplate{
					Type: corev1.ServiceTypeLoadBalancer,
					Metadata: &mariadbv1alpha1.Metadata{
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.214",
						},
					},
				},
				Monitor: mariadbv1alpha1.MaxScaleMonitor{
					Interval: metav1.Duration{Duration: 2 * time.Second},
					Params: map[string]string{
						"auto_failover": "true",
						"auto_rejoin":   "true",
					},
				},
				Auth: mariadbv1alpha1.MaxScaleAuth{
					Generate: ptr.To(true),
					AdminPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{Name: testPwdKey.Name},
							Key:                  testPwdSecretKey,
						},
						Generate: false,
					},
				},
			},
		}
		applyMaxscaleTestConfig(mxs)
		Expect(k8sClient.Create(testCtx, mxs)).To(Succeed())
		DeferCleanup(func() {
			deleteMaxScale(mxsKey, true)
		})
		Eventually(func() bool {
			return strings.Contains(maxScaleState("mariadb-repl-0"), "Master")
		}, testHighTimeout, testInterval).Should(BeTrue())
	})

	It("promotes the replica MaxScale already promoted and rebuilds the old primary", func() {
		By("Suspending the MariaDB so MaxScale fails over first")
		setSuspend(true)

		By("Losing the primary together with its storage")
		deletePVCByPodIndex(mdb, 0)
		deletePodByIndex(mdb, 0)

		By("Expecting MaxScale to promote the replica")
		Eventually(func() bool {
			return strings.Contains(maxScaleState(promotedPod), "Master")
		}, testHighTimeout, testInterval).Should(BeTrue())
		Eventually(func() (bool, error) {
			client := sqlClientForTestPod(mdb, 1)
			defer client.Close()
			readOnly, err := client.IsSystemVariableEnabled(testCtx, "read_only")
			return !readOnly, err
		}, testTimeout, testInterval).Should(BeTrue())

		By("Resuming the MariaDB")
		setSuspend(false)

		By("Expecting the operator to fail over to the promoted replica")
		Eventually(func() string {
			if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
				return ""
			}
			return ptr.Deref(mdb.Status.CurrentPrimary, "")
		}, 5*time.Minute, testInterval).Should(Equal(promotedPod))

		By("Expecting the old primary to be rebuilt as a replica")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
				return false
			}
			return mdb.IsReady() &&
				meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered)
		}, 20*time.Minute, testInterval).Should(BeTrue())
	})
})
