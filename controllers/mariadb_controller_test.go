package controllers

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builders"
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
			Expect(mariaDb.Spec.Image.String()).To(Equal("mariadb:10.7.4"))
			Expect(mariaDb.Spec.Port).To(BeEquivalentTo(3306))
			Expect(mariaDb.Spec.Storage.ClassName).To(Equal("standard"))
			Expect(mariaDb.Spec.Storage.AccessModes).To(ConsistOf(corev1.ReadWriteOnce))

			var sts appsv1.StatefulSet
			By("Expecting to create a StatefulSet eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, mariaDbKey, &sts); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(sts).ToNot(BeNil())

			By("Expecting to create a Service")
			var svc corev1.Service
			Expect(k8sClient.Get(ctx, mariaDbKey, &svc)).To(Succeed())
			Expect(svc).ToNot(BeNil())
		})

		It("Should bootstrap from backup", func() {
			By("Creating BackupMariaDB")
			backupKey := types.NamespacedName{
				Name:      "backup-test",
				Namespace: defaultNamespace,
			}
			backup := databasev1alpha1.BackupMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: databasev1alpha1.BackupMariaDBSpec{
					MariaDBRef: corev1.LocalObjectReference{
						Name: mariaDbName,
					},
					Storage: databasev1alpha1.Storage{
						ClassName: defaultStorageClass,
						Size:      storageSize,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &backup)).To(Succeed())

			By("Expecting BackupMariaDB to be complete eventually")
			Eventually(func() bool {
				var backup databasev1alpha1.BackupMariaDB
				if err := k8sClient.Get(ctx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, timeout, interval).Should(BeTrue())

			By("Creating a MariaDB bootstrapping from backup")
			mariaDbBackupKey := types.NamespacedName{
				Name:      "mariadb-backup",
				Namespace: defaultNamespace,
			}
			mariaDbBackup := databasev1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mariaDbBackupKey.Name,
					Namespace: mariaDbBackupKey.Namespace,
				},
				Spec: databasev1alpha1.MariaDBSpec{
					BootstrapFromBackup: &databasev1alpha1.BootstrapFromBackup{
						BackupRef: corev1.LocalObjectReference{
							Name: backupKey.Name,
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mariaDbRootPwdKey.Name,
						},
						Key: mariaDbRootPwdSecretKey,
					},
					Image: databasev1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					Storage: databasev1alpha1.Storage{
						ClassName: defaultStorageClass,
						Size:      storageSize,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &mariaDbBackup)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				var mariaDbBackup databasev1alpha1.MariaDB
				if err := k8sClient.Get(ctx, mariaDbBackupKey, &mariaDbBackup); err != nil {
					return false
				}
				return mariaDbBackup.IsReady()
			}, timeout, interval).Should(BeTrue())

			By("Deleting MariaDB resources")
			Expect(k8sClient.Delete(ctx, &mariaDbBackup)).To(Succeed())
			var mariaDbPvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, builders.GetPVCKey(&mariaDbBackup), &mariaDbPvc)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &mariaDbPvc)).To(Succeed())

			By("Deleting BackupMariaDB resources")
			Expect(k8sClient.Delete(ctx, &backup)).To(Succeed())
			var backupPvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, backupKey, &backupPvc)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &backupPvc)).To(Succeed())
		})
	})
})
