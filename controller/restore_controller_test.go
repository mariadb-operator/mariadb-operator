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

var _ = Describe("Restore", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should reconcile a Job with BackupRef", func() {
		By("Creating Backup")
		key := types.NamespacedName{
			Name:      "restore-backupref-test",
			Namespace: testNamespace,
		}
		backupKey := types.NamespacedName{
			Name:      fmt.Sprintf("%s-%s", key.Name, "backup"),
			Namespace: testNamespace,
		}
		backup := getBackupWithPVCStorage(backupKey)
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
				JobContainerTemplate: mariadbv1alpha1.JobContainerTemplate{
					Args: []string{"--verbose"},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: corev1.ObjectReference{
						Name: testMdbkey.Name,
					},
					WaitForIt: true,
				},
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
					Annotations: map[string]string{
						"k8s.mariadb.com/test": "test",
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
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(testCtx, key, restore)).To(Succeed())
			var svcAcc corev1.ServiceAccount
			key := restore.Spec.JobPodTemplate.ServiceAccountKey(restore.ObjectMeta)
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
		Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
		Expect(job.ObjectMeta.Annotations).NotTo(BeNil())
		Expect(job.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))

		By("Expecting Restore to complete eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, restore); err != nil {
				return false
			}
			return restore.IsComplete()
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should reconcile a Job with Volume storage", func() {
		By("Creating Backup")
		key := types.NamespacedName{
			Name:      "restore-volume-test",
			Namespace: testNamespace,
		}
		backupKey := types.NamespacedName{
			Name:      fmt.Sprintf("%s-%s", key.Name, "backup"),
			Namespace: testNamespace,
		}
		backup := getBackupWithPVCStorage(backupKey)
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

	It("should reconcile a Job with S3 storage", func() {
		key := types.NamespacedName{
			Name:      "restore-s3-test",
			Namespace: testNamespace,
		}
		testS3BackupRestore(key, "test-restore", "")
	})

	It("should reconcile a Job with S3 storage with prefix", func() {
		key := types.NamespacedName{
			Name:      "restore-s3-test-prefix",
			Namespace: testNamespace,
		}
		testS3BackupRestore(key, "test-restore", "mariadb")
	})
})

func testS3BackupRestore(key types.NamespacedName, bucket, prefix string) {
	backupKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", key.Name, "backup"),
		Namespace: testNamespace,
	}
	backup := getBackupWithS3Storage(backupKey, bucket, prefix)

	By("Creating Backup")
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
				S3: getS3WithBucket(bucket, prefix),
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
}
