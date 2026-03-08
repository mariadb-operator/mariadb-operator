package controller

import (
	"fmt"
	"math/rand/v2"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
			mdb.Spec.PointInTimeRecoveryRef = &mariadbv1alpha1.LocalObjectReference{
				Name: pitr.Name,
			}

			By("Creating MariaDB with replication")
			Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())

			By("Creating Physical Backup")
			Expect(k8sClient.Create(testCtx, physicalBackup)).To(Succeed())

			By("Creating PointInTimeRecovery")
			Expect(k8sClient.Create(testCtx, pitr)).To(Succeed())

			DeferCleanup(func() {
				deleteMariadb(key, false)
				deletePhysicalBackup(key)
				deletePitr(key)
			})
		})

		It("should reconcile MariaDB", func() {
			By("Expecting MariaDB to be ready eventually")
			expectMariadbReady(testCtx, k8sClient, key)

			By("Expecting PhysicalBackup to be ready")
			expectPhysicalBackupReady(physicalBackup)
		})
	})

	Context("With ABS Storage", func() {
		var (
			bucket               = "test-pitr"
			physicalBackupPrefix = "mariadb"
			pitrPrefix           = fmt.Sprintf("mariadb-%d", rand.Int())

			pitr           = buildTestPitr(key, key, withTestPitrABSStorage(bucket, pitrPrefix))
			physicalBackup = buildPhysicalBackupWithABSStorage(key, bucket, physicalBackupPrefix)(key)
		)

		BeforeAll(func() {
			mdb = buildTestMariaDBWithRepl(key)
			applyMariadbTestConfig(mdb)
			mdb.Spec.PointInTimeRecoveryRef = &mariadbv1alpha1.LocalObjectReference{
				Name: pitr.Name,
			}

			By("Creating MariaDB with replication")
			Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())

			By("Creating Physical Backup")
			Expect(k8sClient.Create(testCtx, physicalBackup)).To(Succeed())

			By("Creating PointInTimeRecovery")
			Expect(k8sClient.Create(testCtx, pitr)).To(Succeed())

			DeferCleanup(func() {
				deleteMariadb(key, false)
				deletePhysicalBackup(key)
				deletePitr(key)
			})
		})

		It("should reconcile MariaDB", func() {
			By("Expecting MariaDB to be ready eventually")
			expectMariadbReady(testCtx, k8sClient, key)

			By("Expecting PhysicalBackup to be ready")
			expectPhysicalBackupReady(physicalBackup)
		})
	})
})

// =========================

type testPitrOption func(*mariadbv1alpha1.PointInTimeRecovery)

func withTestPitrS3Storage(bucket, prefix string) testPitrOption {
	return func(p *mariadbv1alpha1.PointInTimeRecovery) {
		p.Spec.PointInTimeRecoveryStorage = mariadbv1alpha1.PointInTimeRecoveryStorage{
			S3: getS3Storage(bucket, prefix),
		}
	}
}

func withTestPitrABSStorage(containerName, prefix string) testPitrOption {
	return func(p *mariadbv1alpha1.PointInTimeRecovery) {
		p.Spec.PointInTimeRecoveryStorage = mariadbv1alpha1.PointInTimeRecoveryStorage{
			AzureBlob: getABSStorage(containerName, prefix),
		}
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
