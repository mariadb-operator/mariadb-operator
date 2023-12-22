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

var _ = Describe("Backup controller", func() {
	Context("When creating a Backup", func() {
		It("Should reconcile a Job", func() {
			By("Creating Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-test",
				Namespace: testNamespace,
			}
			backup := mariadbv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Storage: mariadbv1alpha1.BackupStorage{
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
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			var job batchv1.Job
			By("Expecting to create a Job eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting Job to have mariadb container")
			Expect(job.Spec.Template.Spec.InitContainers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Name": Equal("mariadb"),
				})))

			By("Expecting Job to have mariadb-operator init container")
			Expect(job.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Name": Equal("mariadb-operator"),
				})))

			By("Expecting Backup to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, &backup)).To(Succeed())
		})

		It("Should reconcile a CronJob", func() {
			By("Creating a scheduled Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-test-scheduled",
				Namespace: testNamespace,
			}
			backup := mariadbv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Schedule: &mariadbv1alpha1.Schedule{
						Cron: "*/1 * * * *",
					},
					Storage: mariadbv1alpha1.BackupStorage{
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
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			By("Expecting to create a CronJob eventually")
			Eventually(func() bool {
				var job batchv1.CronJob
				if err := k8sClient.Get(testCtx, backupKey, &job); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting Backup")
			Expect(k8sClient.Delete(testCtx, &backup)).To(Succeed())
		})
	})
})
