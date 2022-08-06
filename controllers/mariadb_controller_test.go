package controllers

import (
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MariaDB controller", func() {
	Context("When creating a MariaDB", func() {
		It("Should reconcile", func() {
			By("Expecting to have spec provided by user and defaults")
			Expect(testMariaDb.Spec.Image.String()).To(Equal("mariadb:10.7.4"))
			Expect(testMariaDb.Spec.Port).To(BeEquivalentTo(3306))
			Expect(testMariaDb.Spec.Storage.ClassName).To(Equal("standard"))
			Expect(testMariaDb.Spec.Storage.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))

			By("Expecting to create a StatefulSet eventually")
			Eventually(func() bool {
				var sts appsv1.StatefulSet
				if err := k8sClient.Get(testCtx, testMariaDbKey, &sts); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, testMariaDbKey, &svc)).To(Succeed())
		})

		It("Should bootstrap from backup", func() {
			By("Creating BackupMariaDB")
			backupKey := types.NamespacedName{
				Name:      "backup-mariadb-test",
				Namespace: testNamespace,
			}
			backup := databasev1alpha1.BackupMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: databasev1alpha1.BackupMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: testMariaDbName,
					},
					Storage: databasev1alpha1.Storage{
						ClassName: testStorageClass,
						Size:      testStorageSize,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			By("Expecting BackupMariaDB to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating a MariaDB bootstrapping from backup")
			backupMariaDbKey := types.NamespacedName{
				Name:      "mariadb-backup",
				Namespace: testNamespace,
			}
			backupMariaDb := databasev1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupMariaDbKey.Name,
					Namespace: backupMariaDbKey.Namespace,
				},
				Spec: databasev1alpha1.MariaDBSpec{
					BootstrapFromBackup: &databasev1alpha1.BootstrapFromBackup{
						BackupRef: corev1.LocalObjectReference{
							Name: backupKey.Name,
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testRootPwdKey.Name,
						},
						Key: testRootPwdSecretKey,
					},
					Image: databasev1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					Storage: databasev1alpha1.Storage{
						ClassName: testStorageClass,
						Size:      testStorageSize,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &backupMariaDb)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupMariaDbKey, &backupMariaDb); err != nil {
					return false
				}
				return backupMariaDb.IsReady()
			}, 60*time.Second, testInterval).Should(BeTrue())

			Expect(k8sClient.Get(testCtx, backupMariaDbKey, &backupMariaDb)).To(Succeed())
			Expect(backupMariaDb.IsBootstrapped()).To(BeTrue())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &backupMariaDb)).To(Succeed())

			By("Deleting BackupMariaDB")
			Expect(k8sClient.Delete(testCtx, &backup)).To(Succeed())
		})
	})

	Context("When creating an invalid MariaDB", func() {
		It("Should report not ready status", func() {
			By("Creating MariaDB")
			invalidMariaDbKey := types.NamespacedName{
				Name:      "mariadb-test-invalid",
				Namespace: testNamespace,
			}
			invalidMariaDb := databasev1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      invalidMariaDbKey.Name,
					Namespace: invalidMariaDbKey.Namespace,
				},
				Spec: databasev1alpha1.MariaDBSpec{
					Image: databasev1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					Storage: databasev1alpha1.Storage{
						ClassName: testStorageClass,
						Size:      testStorageSize,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &invalidMariaDb)).To(Succeed())

			By("Expecting not ready status consistently")
			Consistently(func() bool {
				if err := k8sClient.Get(testCtx, invalidMariaDbKey, &invalidMariaDb); err != nil {
					return false
				}
				return !invalidMariaDb.IsReady()
			}, 5*time.Second, testInterval)

			Expect(k8sClient.Get(testCtx, invalidMariaDbKey, &invalidMariaDb)).To(Succeed())
			Expect(invalidMariaDb.IsBootstrapped()).To(BeFalse())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &invalidMariaDb)).To(Succeed())
		})
	})

	Context("When bootstrapping from a non existing backup", func() {
		It("Should report not ready status", func() {
			By("Creating MariaDB")
			noBackupMariaDbKey := types.NamespacedName{
				Name:      "mariadb-test-no-backup",
				Namespace: testNamespace,
			}
			noBackupMariaDb := databasev1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noBackupMariaDbKey.Name,
					Namespace: noBackupMariaDbKey.Namespace,
				},
				Spec: databasev1alpha1.MariaDBSpec{
					BootstrapFromBackup: &databasev1alpha1.BootstrapFromBackup{
						BackupRef: corev1.LocalObjectReference{
							Name: "foo",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testRootPwdKey.Name,
						},
						Key: testRootPwdSecretKey,
					},
					Image: databasev1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					Storage: databasev1alpha1.Storage{
						ClassName: testStorageClass,
						Size:      testStorageSize,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &noBackupMariaDb)).To(Succeed())

			By("Expecting not ready status consistently")
			Consistently(func() bool {
				if err := k8sClient.Get(testCtx, noBackupMariaDbKey, &noBackupMariaDb); err != nil {
					return false
				}
				return !noBackupMariaDb.IsReady()
			}, 5*time.Second, testInterval)

			Expect(k8sClient.Get(testCtx, noBackupMariaDbKey, &noBackupMariaDb)).To(Succeed())
			Expect(noBackupMariaDb.IsBootstrapped()).To(BeFalse())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &noBackupMariaDb)).To(Succeed())
		})
	})
})
