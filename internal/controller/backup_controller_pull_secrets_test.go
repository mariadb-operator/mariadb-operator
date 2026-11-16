package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A pull Secret declared on both the MariaDB and the Backup is listed once in the Backup Pods: the API server stores
// a duplicate as is and warns about it on every write of the CronJob.
var _ = Describe("Backup image pull secrets", Ordered, func() {
	var (
		mdbKey     = types.NamespacedName{Name: "mariadb-pull-secrets", Namespace: testNamespace}
		backupKey  = types.NamespacedName{Name: "backup-pull-secrets", Namespace: testNamespace}
		pullSecret = mariadbv1alpha1.LocalObjectReference{Name: "mariadb-registry"}
	)

	BeforeAll(func() {
		mdb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{Name: mdbKey.Name, Namespace: mdbKey.Namespace},
			Spec: mariadbv1alpha1.MariaDBSpec{
				MariaDBPodTemplate: mariadbv1alpha1.MariaDBPodTemplate{
					ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{pullSecret},
				},
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
			},
		}
		Expect(k8sClient.Create(testCtx, mdb)).To(Succeed())
		DeferCleanup(func() {
			deleteMariadb(mdbKey, true)
		})

		backup := &mariadbv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: backupKey.Name, Namespace: backupKey.Namespace},
			Spec: mariadbv1alpha1.BackupSpec{
				JobPodTemplate: mariadbv1alpha1.JobPodTemplate{
					ImagePullSecrets: []mariadbv1alpha1.LocalObjectReference{pullSecret},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{Name: mdbKey.Name},
				},
				Schedule: &mariadbv1alpha1.Schedule{Cron: "0 3 * * *", Suspend: true},
				Storage: mariadbv1alpha1.BackupStorage{
					Volume: &mariadbv1alpha1.StorageVolumeSource{
						EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
					},
				},
			},
		}
		Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, backup))).To(Succeed())
		})
	})

	It("lists a shared pull Secret once in the backup CronJob", func() {
		var cronJob batchv1.CronJob
		Eventually(func() error {
			return k8sClient.Get(testCtx, backupKey, &cronJob)
		}, testTimeout, testInterval).Should(Succeed())
		Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets).To(Equal(
			[]corev1.LocalObjectReference{{Name: pullSecret.Name}},
		))
	})
})
