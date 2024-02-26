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

			By("Creating Restore")
			restore := &mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Args: []string{"--verbose"},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
						},
						WaitForIt: true,
					},
					InheritMetadata: &mariadbv1alpha1.InheritMetadata{
						Labels: map[string]string{
							"mariadb.mmontes.io/test": "test",
						},
						Annotations: map[string]string{
							"mariadb.mmontes.io/test": "test",
						},
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: backup.Name,
						},
						TargetRecoveryTime: &metav1.Time{Time: time.Now()},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, restore)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, restore)).To(Succeed())
			})

			By("Expecting to create a ServiceAccount eventually")
			Eventually(func() bool {
				var svcAcc corev1.ServiceAccount
				key := restore.Spec.PodTemplate.ServiceAccountKey(restore.ObjectMeta)
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

			By("Expecting Job to have metadata")
			Expect(job.ObjectMeta.Labels).NotTo(BeNil())
			Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))
			Expect(job.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(job.ObjectMeta.Annotations).To(HaveKeyWithValue("mariadb.mmontes.io/test", "test"))

			By("Expecting Restore to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
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
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
			})

			By("Creating Restore")
			restore := &mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
						},
						WaitForIt: true,
					},
					RestoreSource: mariadbv1alpha1.RestoreSource{
						S3: testS3WithBucket(bucket),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, restore)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, restore)).To(Succeed())
			})

			By("Expecting Restore to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
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

			By("Creating Restore")
			restore := &mariadbv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: mariadbv1alpha1.RestoreSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
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
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, restore)).To(Succeed())
			})

			By("Expecting Restore to complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, restore); err != nil {
					return false
				}
				return restore.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())
		})
	})
})
