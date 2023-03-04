package controllers

import (
	"time"

	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MariaDB controller", func() {
	Context("When creating a MariaDB", func() {
		It("Should reconcile", func() {
			By("Expecting to have spec provided by user and defaults")
			Expect(testMariaDb.Spec.Image.String()).To(Equal("mariadb:10.7.4"))
			Expect(testMariaDb.Spec.Port).To(BeEquivalentTo(3306))

			By("Expecting to create a ConfigMap eventually")
			Eventually(func() bool {
				var cm corev1.ConfigMap
				if err := k8sClient.Get(testCtx, configMapMariaDBKey(&testMariaDb), &cm); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a StatefulSet eventually")
			Eventually(func() bool {
				var sts appsv1.StatefulSet
				if err := k8sClient.Get(testCtx, testMariaDbKey, &sts); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting to create a Service eventually")
			Eventually(func() bool {
				var svc corev1.Service
				if err := k8sClient.Get(testCtx, testMariaDbKey, &svc); err != nil {
					return false
				}
				return true
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting Connection to be ready eventually")
			Eventually(func() bool {
				var conn mariadbv1alpha1.Connection
				if err := k8sClient.Get(testCtx, connectionKey(&testMariaDb), &conn); err != nil {
					return false
				}
				return conn.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())
		})

		It("Should bootstrap from backup", func() {
			By("Creating Backup")
			backupKey := types.NamespacedName{
				Name:      "backup-mariadb-test",
				Namespace: testNamespace,
			}
			backup := mariadbv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: mariadbv1alpha1.BackupSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &testStorageClassName,
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

			By("Expecting Backup to be complete eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, backupKey, &backup); err != nil {
					return false
				}
				return backup.IsComplete()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Creating a MariaDB bootstrapping from backup")
			bootstrapMariaDBKey := types.NamespacedName{
				Name:      "mariadb-backup",
				Namespace: testNamespace,
			}
			bootstrapMariaDB := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstrapMariaDBKey.Name,
					Namespace: bootstrapMariaDBKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: backupKey.Name,
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Image: mariadbv1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &testStorageClassName,
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
			}
			Expect(k8sClient.Create(testCtx, &bootstrapMariaDB)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, bootstrapMariaDBKey, &bootstrapMariaDB); err != nil {
					return false
				}
				return bootstrapMariaDB.IsReady()
			}, 60*time.Second, testInterval).Should(BeTrue())

			Expect(k8sClient.Get(testCtx, bootstrapMariaDBKey, &bootstrapMariaDB)).To(Succeed())
			Expect(bootstrapMariaDB.IsBootstrapped()).To(BeTrue())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &bootstrapMariaDB)).To(Succeed())

			By("Deleting Backup")
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
			invalidMariaDb := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      invalidMariaDbKey.Name,
					Namespace: invalidMariaDbKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: mariadbv1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &testStorageClassName,
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
			noBackupKey := types.NamespacedName{
				Name:      "mariadb-test-no-backup",
				Namespace: testNamespace,
			}
			noBackup := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noBackupKey.Name,
					Namespace: noBackupKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.RestoreSource{
						BackupRef: &v1.LocalObjectReference{
							Name: "foo",
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Image: mariadbv1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &testStorageClassName,
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
			}
			Expect(k8sClient.Create(testCtx, &noBackup)).To(Succeed())

			By("Expecting not ready status consistently")
			Consistently(func() bool {
				if err := k8sClient.Get(testCtx, noBackupKey, &noBackup); err != nil {
					return false
				}
				return !noBackup.IsReady()
			}, 5*time.Second, testInterval)

			Expect(k8sClient.Get(testCtx, noBackupKey, &noBackup)).To(Succeed())
			Expect(noBackup.IsBootstrapped()).To(BeFalse())

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &noBackup)).To(Succeed())
		})
	})

	Context("When updating a MariaDB", func() {
		It("Should reconcile", func() {
			By("Performing update")
			updateMariaDBKey := types.NamespacedName{
				Name:      "test-update-mariadb",
				Namespace: testNamespace,
			}
			updateMariaDB := mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      updateMariaDBKey.Name,
					Namespace: updateMariaDBKey.Namespace,
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Image: mariadbv1alpha1.Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &testStorageClassName,
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
			}
			Expect(k8sClient.Create(testCtx, &updateMariaDB)).To(Succeed())
			updateMariaDB.Spec.Port = 3307
			Expect(k8sClient.Update(testCtx, &updateMariaDB)).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, updateMariaDBKey, &updateMariaDB); err != nil {
					return false
				}
				return updateMariaDB.IsReady()
			}, testTimeout, testInterval).Should(BeTrue())

			By("Expecting port to be updated in StatefulSet")
			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(testCtx, updateMariaDBKey, &sts)).To(Succeed())
			containerPort, err := builder.StatefulSetPort(&sts)
			Expect(err).NotTo(HaveOccurred())
			Expect(containerPort.ContainerPort).To(BeEquivalentTo(3307))

			By("Expecting port to be updated in Service")
			var svc corev1.Service
			Expect(k8sClient.Get(testCtx, updateMariaDBKey, &svc)).To(Succeed())
			svcPort, err := builder.GetServicePort(&svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svcPort.Port).To(BeEquivalentTo(3307))

			By("Deleting MariaDB")
			Expect(k8sClient.Delete(testCtx, &updateMariaDB)).To(Succeed())
		})
	})
})
