package controller

import (
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Restore controller", func() {
	Context("When creating a restore", func() {
		It("Should reconcile a Job with BackupRef", func() {
			By("Creating Backup")
			key := types.NamespacedName{
				Name:      "restore-backupref-test",
				Namespace: testNamespace,
			}
			backupKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", key.Name, "backup"),
				Namespace: testNamespace,
			}
			backup := testBackupWithPVCStorage(backupKey)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating Restore")
			restore := &mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: backup.Name,
						},
						TargetRecoveryTime: &metav1.Time{Time: time.Now()},
					},
					Args: []string{"--verbose"},
				},
			}
			Expect(k8sClient.Create(testCtx, restore)).To(Succeed())

			var job batchv1.Job
			By("Expecting to create a Job eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting Job to have mariadb-operator init container")
			Expect(job.Spec.Template.Spec.InitContainers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Name": Equal("mariadb-operator"),
				})))

			By("Expecting Job to have mariadb container")
			Expect(job.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Name": Equal("mariadb"),
				})))

			By("Expecting Restore to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())

			By("Deleting Restore")
			Expect(k8sClient.Delete(testCtx, restore)).To(Succeed())
		})

		It("Should reconcile a Job with S3 storage", func() {
			By("Creating Backup")
			key := types.NamespacedName{
				Name:      "restore-s3-test",
				Namespace: testNamespace,
			}
			backupKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", key.Name, "backup"),
				Namespace: testNamespace,
			}
			bucket := "test-restore"
			backup := testBackupWithS3Storage(backupKey, bucket)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating Restore")
			restore := &mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						S3: testS3WithBucket(bucket),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, restore)).To(Succeed())

			By("Expecting Restore to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())

			By("Deleting Restore")
			Expect(k8sClient.Delete(testCtx, restore)).To(Succeed())
		})

		It("Should reconcile a Job with Volume storage", func() {
			By("Creating Backup")
			key := types.NamespacedName{
				Name:      "restore-volume-test",
				Namespace: testNamespace,
			}
			backupKey := types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", key.Name, "backup"),
				Namespace: testNamespace,
			}
			backup := testBackupWithPVCStorage(backupKey)
			Expect(k8sClient.Create(testCtx, backup)).To(Succeed())

			By("Expecting Backup to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating Restore")
			restore := &mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: backupKey.Name,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, restore)).To(Succeed())

			By("Expecting Restore to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())

			By("Deleting Restore")
			Expect(k8sClient.Delete(testCtx, restore)).To(Succeed())
		})
	})
})
