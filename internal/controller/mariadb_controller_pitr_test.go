package controller

import (
	"fmt"
	"math/rand/v2"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("MariaDB PITR with Replication", Ordered, func() {
	var (
		// Used for MariaDB, PITR, PhysicalBackup
		key = types.NamespacedName{
			Name:      "mariadb-repl",
			Namespace: testNamespace,
		}
		mdb *mariadbv1alpha1.MariaDB
	)

	Context("With S3 Storage", func() {
		var (
			bucket               = "test-pitr"
			physicalBackupPrefix = "mariadb"
			pitrPrefix           = fmt.Sprintf("mariadb-%d", rand.Int())

			pitr           = buildTestPitr(key, key, withTestPitrS3Storage(bucket, pitrPrefix))
			physicalBackup = buildPhysicalBackupWithS3Storage(key, bucket, physicalBackupPrefix)(key)
		)

		BeforeAll(func() {
			mdb = buildTestMariaDBWithRepl(key)
			applyMariadbTestConfig(mdb)

			// @NOTE: `log_slave_update` we add so the replica will have binlogs to replicate when it switches over.
			mdb.Spec.MyCnf = ptr.To(`[mariadb]
max_binlog_size = 4096
log_slave_update = 1`)

			By("Creating MariaDB with replication")
			Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())

			DeferCleanup(func() {
				deleteMariadb(key, false)
				deletePhysicalBackup(key)
			})
		})

		AfterAll(func() {
			deletePitr(key)
		})

		It(fmt.Sprintf("should reconcile in bucket %s", pitrPrefix), func() {
			By("Expecting MariaDB to be ready eventually")
			expectMariadbReady(testCtx, k8sClient, key)

			By("Generating binlogs before adding PITR")
			generateBinlogs(mdb)

			By("Updating MariaDB")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				mdb.Spec.PointInTimeRecoveryRef = &mariadbv1alpha1.LocalObjectReference{
					Name: pitr.Name,
				}

				return k8sClient.Update(testCtx, mdb) == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating Physical Backup")
			Expect(k8sClient.Create(testCtx, physicalBackup)).To(Succeed())

			By("Expecting PhysicalBackup to be ready")
			expectPhysicalBackupReady(physicalBackup)

			By("Creating PointInTimeRecovery")
			Expect(k8sClient.Create(testCtx, pitr)).To(Succeed())

			Eventually(func(g Gomega) bool {
				s3Client, err := minio.NewMinioClientFromS3Config(testCtx, *refresolver.New(k8sClient), pitr.Spec.S3, "", pitr.Namespace)
				g.Expect(err).To(Succeed())

				items, err := s3Client.ListObjectsWithOptions(testCtx)
				g.Expect(err).To(Succeed())

				// Index and server-%d gets created
				g.Expect(items).To(HaveLen(3))
				return true
			}, testHighTimeout, testInterval).To(BeTrue())
		})
	})

	PContext("With ABS Storage", func() {
		It("should reconcile", func() {
			By("Creating MariaDB with replication")
			// pitr := buildTestPitr(key, key, withTestPitrABSStorage(bucket, prefix))
			// physicalBackup := buildPhysicalBackupWithABSStorage(key, bucket, prefix)(key)
			Expect(true).To(BeFalse(), "ABS storage is not implemented yet")
		})
	})
})

// Does not work due to how LastRecoverableTime is calculated.
// func testPointInTimeRecovery(pitr *mariadbv1alpha1.PointInTimeRecovery) {
// 	key := client.ObjectKeyFromObject(pitr)
// 	By("Expecting Point In Time Recovery to be ready eventually")
// 	Eventually(func(g Gomega) bool {
// 		g.Expect(k8sClient.Get(testCtx, key, pitr)).To(Succeed())
// 		return pitr.Status.LastRecoverableTime != nil
// 	}, testHighTimeout, testInterval).Should(BeTrue())
// }

func generateBinlogs(mdb *mariadbv1alpha1.MariaDB) {
	primaryIndex := ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, 0)
	By("Creating database")
	executeSqlInPodByIndex(mdb, primaryIndex, "CREATE DATABASE IF NOT EXISTS pitr;")

	By("Creating table")
	executeSqlInPodByIndex(mdb, primaryIndex, "CREATE TABLE IF NOT EXISTS pitr.test (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(255));")

	By("Generating binlogs")
	for i := range 500 {
		executeSqlInPodByIndex(mdb, primaryIndex, fmt.Sprintf("INSERT INTO pitr.test (name) VALUES ('test-%d');", i))
	}
}

// =========================

type testPitrOption func(*mariadbv1alpha1.PointInTimeRecovery)

func withTestPitrS3Storage(bucket, prefix string) testPitrOption {
	return func(p *mariadbv1alpha1.PointInTimeRecovery) {
		p.Spec.S3 = *getS3Storage(bucket, prefix)
	}
}

func buildTestPitr(pitrKey, physicalBackupKey types.NamespacedName, opts ...testPitrOption) *mariadbv1alpha1.PointInTimeRecovery {
	p := &mariadbv1alpha1.PointInTimeRecovery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pitrKey.Name,
			Namespace: pitrKey.Namespace,
		},
		Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
			PhysicalBackupRef: mariadbv1alpha1.LocalObjectReference{
				Name: physicalBackupKey.Name,
			},
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}
