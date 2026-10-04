package controller

import (
	"fmt"
	"strings"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A primary that loses its PVC is failed over on paper by the PVC-change failover: status and spec
// move to the candidate, but no SQL runs on it. The candidate must still be promoted while the old
// primary is being rebuilt, the rebuilt node must never be elected primary, and a switch back to the
// original index afterwards must not lose any write made in between.
var _ = Describe("MariaDB primary PVC failover", Ordered, func() {
	var (
		mdbKey    = types.NamespacedName{Name: "mariadb-repl", Namespace: testNamespace}
		mxsKey    = types.NamespacedName{Name: "maxscale-repl", Namespace: testNamespace}
		backupKey = types.NamespacedName{Name: "mariadb-repl-pvc-failover-recovery", Namespace: testNamespace}
		mdb       *mariadbv1alpha1.MariaDB
		mxs       *mariadbv1alpha1.MaxScale
		sawStuck  bool
	)

	const (
		markerDB    = "pvcfailoverdb"
		markerTable = "marker"
	)
	rowExists := func(podIndex int, id int) (bool, error) {
		clientSet := sql.NewClientSet(mdb, refresolver.New(k8sClient))
		sqlClient, err := clientSet.ClientForIndex(testCtx, podIndex)
		if err != nil {
			return false, err
		}
		defer sqlClient.Close()
		return sqlClient.Exists(testCtx, fmt.Sprintf("SELECT id FROM %s.%s WHERE id = ?;", markerDB, markerTable), id)
	}

	snapshot := func(tag string) (map[string]string, string) {
		states := map[string]string{}
		var m mariadbv1alpha1.MaxScale
		var d mariadbv1alpha1.MariaDB
		_ = k8sClient.Get(testCtx, mxsKey, &m)
		_ = k8sClient.Get(testCtx, mdbKey, &d)
		for _, s := range m.Status.Servers {
			states[s.Name] = s.State
		}
		primary := ptr.Deref(d.Status.CurrentPrimary, "")
		recovered := meta.FindStatusCondition(d.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered)
		rec := "<none>"
		if recovered != nil {
			rec = string(recovered.Status) + "/" + recovered.Reason
		}
		if m.Status.NoPrimaryServerSince != nil {
			sawStuck = true
		}
		GinkgoWriter.Printf("[%s %s] mdbPrimary=%s mdbReady=%v recovered=%s mxsPrimary=%s noPrimarySince=%v servers=%v\n",
			tag, time.Now().UTC().Format("15:04:05"), primary, d.IsReady(), rec,
			ptr.Deref(m.Status.PrimaryServer, ""), m.Status.NoPrimaryServerSince, states)
		return states, primary
	}

	BeforeAll(func() {
		backup := buildPhysicalBackupWithS3Storage(mdbKey, "test-replication-recovery", "pvc-failover")(backupKey)
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

		mxs = &mariadbv1alpha1.MaxScale{
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
			states, _ := snapshot("setup")
			return strings.Contains(states["mariadb-repl-0"], "Master")
		}, testHighTimeout, 10*time.Second).Should(BeTrue())
	})

	It("promotes the candidate, rebuilds the old primary and switches back without losing data", func() {
		snapshot("start")

		By("Losing the primary together with its storage")
		deletePVCByPodIndex(mdb, 0)
		deletePodByIndex(mdb, 0)

		By("Expecting the failover candidate to become the MaxScale Master")
		Eventually(func() bool {
			states, primary := snapshot("phase1")
			return primary == "mariadb-repl-1" && strings.Contains(states["mariadb-repl-1"], "Master")
		}, 10*time.Minute, 10*time.Second).Should(BeTrue())

		By("Writing a marker row on the failed-over primary mariadb-repl-1")
		executeSqlInPodByIndex(mdb, 1, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", markerDB))
		executeSqlInPodByIndex(mdb, 1, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id INT PRIMARY KEY);", markerDB, markerTable))
		executeSqlInPodByIndex(mdb, 1, fmt.Sprintf("INSERT INTO %s.%s VALUES (1);", markerDB, markerTable))
		GinkgoWriter.Printf("MARKER row 1 written on mariadb-repl-1 at %s\n", time.Now().UTC().Format("15:04:05"))

		By("Expecting the old primary to be recovered as a replica")
		Eventually(func() bool {
			var d mariadbv1alpha1.MariaDB
			if err := k8sClient.Get(testCtx, mdbKey, &d); err != nil {
				return false
			}
			snapshot("phase2")
			return meta.IsStatusConditionTrue(d.Status.Conditions, mariadbv1alpha1.ConditionTypeReplicaRecovered)
		}, 20*time.Minute, 15*time.Second).Should(BeTrue())

		By("Writing a second marker row on mariadb-repl-1 after recovery")
		executeSqlInPodByIndex(mdb, 1, fmt.Sprintf("INSERT INTO %s.%s VALUES (2);", markerDB, markerTable))

		By("Reinstating podIndex 0 as GitOps would and expecting a switchover back")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
				return false
			}
			mdb.Spec.Replication.Primary.PodIndex = ptr.To(0)
			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())
		Eventually(func() bool {
			_, primary := snapshot("phase3")
			return primary == "mariadb-repl-0"
		}, 15*time.Minute, 10*time.Second).Should(BeTrue())

		By("Expecting MaxScale to route writes to the original primary")
		start := time.Now()
		Eventually(func() bool {
			states, _ := snapshot("phase4")
			st := states["mariadb-repl-0"]
			return strings.Contains(st, "Master") && !strings.Contains(st, "Maintenance")
		}, 15*time.Minute, 10*time.Second).Should(BeTrue())
		GinkgoWriter.Printf("PHASE4 recovered in %s, pool was stuck without a Master at some point: %v\n",
			time.Since(start).Round(time.Second), sawStuck)

		By("Verifying no data was lost across failover, recovery and switch-back")
		for _, idx := range []int{0, 1} {
			for _, id := range []int{1, 2} {
				Eventually(func() bool {
					ok, err := rowExists(idx, id)
					if err != nil {
						GinkgoWriter.Printf("rowExists(pod %d, id %d) err: %v\n", idx, id, err)
					}
					return err == nil && ok
				}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "marker row %d missing on mariadb-repl-%d", id, idx)
			}
		}
		GinkgoWriter.Printf("DATA-CHECK ok: marker rows 1 and 2 present on both Pods\n")

		var d mariadbv1alpha1.MariaDB
		Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(mdb), &d)).To(Succeed())
	})
})
