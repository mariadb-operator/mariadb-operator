package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Backup", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should reconcile a Job with PVC storage", func() {
		key := types.NamespacedName{
			Name:      "backup-pvc-test",
			Namespace: testNamespace,
		}
		backup := getBackupWithPVCStorage(key)
		testBackup(backup)
	})

	It("should reconcile a Job with Volume storage", func() {
		key := types.NamespacedName{
			Name:      "backup-volume-test",
			Namespace: testNamespace,
		}
		backup := getBackupWithVolumeStorage(key)
		testBackup(backup)
	})

	It("should reconcile a Job with S3 storage", func() {
		key := types.NamespacedName{
			Name:      "backup-s3-test",
			Namespace: testNamespace,
		}
		testS3Backup(key, "test-backup", "")
	})

	It("should reconcile a Job with S3 storage with prefix", func() {
		key := types.NamespacedName{
			Name:      "backup-s3-test-prefix",
			Namespace: testNamespace,
		}
		testS3Backup(key, "test-backup", "mariadb")
	})
})

func testBackup(backup *mariadbv1alpha1.Backup) {
	key := client.ObjectKeyFromObject(backup)

	By("Creating Backup")
	Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
	})

	By("Expecting to create a ServiceAccount eventually")
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(testCtx, key, backup)).To(Succeed())
		var svcAcc corev1.ServiceAccount
		key := backup.Spec.JobPodTemplate.ServiceAccountKey(backup.ObjectMeta)
		g.Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())

		g.Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
		g.Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
		g.Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
		g.Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	var job batchv1.Job
	By("Expecting to create a Job eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, &job); err != nil {
			return false
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting Job to have mariadb init container")
	Expect(job.Spec.Template.Spec.InitContainers).To(ContainElement(MatchFields(IgnoreExtras,
		Fields{
			"Name": Equal("mariadb"),
		})))

	By("Expecting Job to have mariadb-operator container")
	Expect(job.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
		Fields{
			"Name": Equal("mariadb-operator"),
		})))

	By("Expecting Job to have metadata")
	Expect(job.ObjectMeta.Labels).NotTo(BeNil())
	Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
	Expect(job.ObjectMeta.Annotations).NotTo(BeNil())
	Expect(job.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))

	By("Expecting Backup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		return backup.IsComplete()
	}, testTimeout, testInterval).Should(BeTrue())
}

func testS3Backup(key types.NamespacedName, bucket, prefix string) {
	backup := getBackupWithS3Storage(key, bucket, prefix)

	By("Creating Backup with S3 storage")
	Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
	})

	By("Expecting Backup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		return backup.IsComplete()
	}, testTimeout, testInterval).Should(BeTrue())
}
