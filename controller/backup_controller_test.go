package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func testBackupWithStorage(key types.NamespacedName, storage mariadbv1alpha1.BackupStorage) *mariadbv1alpha1.Backup {
	return &mariadbv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.BackupSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name: testMariaDbName,
				},
				WaitForIt: true,
			},
			Storage: storage,
		},
	}
}

func testBackupWithPVCStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("100Mi"),
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		},
	})
}

func testBackupWithS3Storage(key types.NamespacedName, bucket string) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		S3: testS3WithBucket(bucket),
	})
}

func testBackupWithVolumeStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		Volume: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}

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

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
		})

		It("Should reconcile a CronJob with PVC storage", func() {
			By("Creating a scheduled Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-pvc-scheduled-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithPVCStorage(backupKey)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting to create a CronJob eventually")
			Eventually(func() bool {
				var job batchv1.CronJob
				if err := k8sClient.Get(testCtx, backupKey, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
		})

		It("Should reconcile a Job with S3 storage", func() {
			By("Creating Backup with S3 storage")
			backupKey := types.NamespacedName{
				Name:      "backup-s3-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithS3Storage(backupKey, "test-backup")
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
		})

		It("Should reconcile a Job with Volume storage", func() {
			By("Creating Backup with Volume storage")
			backupKey := types.NamespacedName{
				Name:      "backup-volume-test",
				Namespace: testNamespace,
			}
			backup := testBackupWithVolumeStorage(backupKey)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
		})
	})
})
