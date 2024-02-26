package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Backup controller", func() {
	Context("When creating a Backup", func() {
		It("Should reconcile a Job with PVC storage", func() {
			By("Creating Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-pvc-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithPVCStorage(backupKey)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			By("Expecting to create a ServiceAccount eventually")
			Eventually(func() bool {
				var svcAcc corev1.ServiceAccount
				key := backup.Spec.PodTemplate.ServiceAccountKey(backup.ObjectMeta)
				if err := k8sClient.Get(testCtx, key, &svcAcc); err != nil {
					return false
				}
				Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
				Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))
				Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
				Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			var job batchv1.Job
			By("Expecting to create a Job eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &job); err != nil {
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
			Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))
			Expect(job.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(job.ObjectMeta.Annotations).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
		})

		It("Should reconcile a CronJob with PVC storage", func() {
			By("Creating a scheduled Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-pvc-scheduled-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithPVCStorage(backupKey)
			backupWithSchedule := &mariadbv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: backup.Spec.MariaDBRef,
					Schedule: &mariadbv1alpha1.Schedule{
						Cron: "*/1 * * * *",
					},
					Storage: backup.Spec.Storage,
				},
			}
			Expect(k8sClient.Create(testCtx, backupWithSchedule)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backupWithSchedule)).To(Succeed())
			})

			By("Expecting to create a CronJob eventually")
			Eventually(func() bool {
				var job batchv1.CronJob
				if err := k8sClient.Get(testCtx, backupKey, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())
		})

		It("Should reconcile a Job with S3 storage", func() {
			By("Creating Backup with S3 storage")
			backupKey := types.NamespacedName{
				Name:      "backup-s3-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithS3Storage(backupKey, "test-backup")
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
		})

		It("Should reconcile a Job with Volume storage", func() {
			By("Creating Backup with Volume storage")
			backupKey := types.NamespacedName{
				Name:      "backup-volume-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithVolumeStorage(backupKey)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
		})
	})
})
