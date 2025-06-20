package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/pkg/volumesnapshot"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PhysicalBackup", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	DescribeTable("Creating a PhysicalBackup",
		func(
			resourceName string,
			builderFn func(types.NamespacedName) *mariadbv1alpha1.PhysicalBackup,
			testFn func(*mariadbv1alpha1.PhysicalBackup),
		) {
			key := types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}
			backup := builderFn(key)
			testFn(backup)
		},
		Entry(
			"should reconcile a Job with PVC storage",
			"physicalbackup-job-pvc-test",
			getPhysicalBackupWithPVCStorage,
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with Volume storage",
			"physicalbackup-job-volume-test",
			getPhysicalBackupWithVolumeStorage,
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage",
			"physicalbackup-job-s3-test",
			buildPhysicalBackupWithS3Storage("test-physicalbackup", ""),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage with prefix",
			"physicalbackup-job-s3-prefix-test",
			buildPhysicalBackupWithS3Storage("test-physicalbackup", "mariadb"),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage and bzip2 compression",
			"physicalbackup-job-s3-bzip2-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage("test-physicalbackup", ""),
				decoratePhysicalBackupWithBzip2Compression,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage and gzip compression",
			"physicalbackup-job-s3-gzip-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage("test-physicalbackup", ""),
				decoratePhysicalBackupWithGzipCompression,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage and staging storage",
			"physicalbackup-job-s3-staging-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage("test-physicalbackup", ""),
				decoratePhysicalBackupWithStagingStorage,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a VolumeSnapshot",
			"physicalbackup-volumesnapshot-test",
			getPhysicalBackupWithVolumeSnapshotStorage,
			testPhysicalBackup,
		),
	)
})

func testPhysicalBackup(backup *mariadbv1alpha1.PhysicalBackup) {
	By("Creating PhysicalBackup")
	Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
	})

	if backup.Spec.Storage.VolumeSnapshot != nil {
		testPhysicalBackupVolumeSnapshot(backup)
	} else {
		testPhysicalBackupJob(backup)
	}
}

func testPhysicalBackupJob(backup *mariadbv1alpha1.PhysicalBackup) {
	key := client.ObjectKeyFromObject(backup)

	By("Expecting to create a ServiceAccount eventually")
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(testCtx, key, backup)).To(Succeed())
		var svcAcc corev1.ServiceAccount
		key := backup.Spec.PhysicalBackupPodTemplate.ServiceAccountKey(backup.ObjectMeta)
		g.Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	var jobList *batchv1.JobList
	By("Expecting to create a Job eventually")
	Eventually(func() bool {
		var err error
		jobList, err = job.ListJobs(testCtx, k8sClient, backup)
		if err != nil {
			return false
		}
		return len(jobList.Items) > 0
	}, testTimeout, testInterval).Should(BeTrue())

	job := jobList.Items[0]
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

	By("Expecting PhysicalBackup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		return backup.IsComplete()
	}, testTimeout, testInterval).Should(BeTrue())
}

func testPhysicalBackupVolumeSnapshot(backup *mariadbv1alpha1.PhysicalBackup) {
	key := client.ObjectKeyFromObject(backup)

	By("Expecting to create a VolumeSnapshot eventually")
	Eventually(func() bool {
		volumeSnapshotList, err := volumesnapshot.ListVolumeSnapshots(testCtx, k8sClient, backup)
		if err != nil {
			return false
		}
		return len(volumeSnapshotList.Items) > 0
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting PhysicalBackup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		return backup.IsComplete()
	}, testTimeout, testInterval).Should(BeTrue())
}

func getPhysicalBackupWithStorage(key types.NamespacedName, storage mariadbv1alpha1.PhysicalBackupStorage) *mariadbv1alpha1.PhysicalBackup {
	return &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: testMdbkey.Name,
				},
				WaitForIt: true,
			},
			Storage: storage,
		},
	}
}

func getPhysicalBackupWithPVCStorage(key types.NamespacedName) *mariadbv1alpha1.PhysicalBackup {
	return getPhysicalBackupWithStorage(key, mariadbv1alpha1.PhysicalBackupStorage{
		PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
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

func getPhysicalBackupWithS3Storage(key types.NamespacedName, bucket, prefix string) *mariadbv1alpha1.PhysicalBackup {
	return getPhysicalBackupWithStorage(key, mariadbv1alpha1.PhysicalBackupStorage{
		S3: getS3WithBucket(bucket, prefix),
	})
}

func getPhysicalBackupWithVolumeStorage(key types.NamespacedName) *mariadbv1alpha1.PhysicalBackup {
	return getPhysicalBackupWithStorage(key, mariadbv1alpha1.PhysicalBackupStorage{
		Volume: &mariadbv1alpha1.StorageVolumeSource{
			EmptyDir: &mariadbv1alpha1.EmptyDirVolumeSource{},
		},
	})
}

func getPhysicalBackupWithVolumeSnapshotStorage(key types.NamespacedName) *mariadbv1alpha1.PhysicalBackup {
	return getPhysicalBackupWithStorage(key, mariadbv1alpha1.PhysicalBackupStorage{
		VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
			VolumeSnapshotClassName: "csi-hostpath-snapclass",
		},
	})
}

// func decoratePhysicalBackupWithSchedule(backup *mariadbv1alpha1.PhysicalBackup) *mariadbv1alpha1.PhysicalBackup {
// 	backup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{Cron: "*/5 * * * *"}
// 	backup.Spec.Schedule.Immediate = ptr.To(true)
// 	return backup
// }

func decoratePhysicalBackupWithGzipCompression(backup *mariadbv1alpha1.PhysicalBackup) *mariadbv1alpha1.PhysicalBackup {
	backup.Spec.Compression = mariadbv1alpha1.CompressGzip
	return backup
}

func decoratePhysicalBackupWithBzip2Compression(backup *mariadbv1alpha1.PhysicalBackup) *mariadbv1alpha1.PhysicalBackup {
	backup.Spec.Compression = mariadbv1alpha1.CompressBzip2
	return backup
}

func decoratePhysicalBackupWithStagingStorage(backup *mariadbv1alpha1.PhysicalBackup) *mariadbv1alpha1.PhysicalBackup {
	backup.Spec.StagingStorage = &mariadbv1alpha1.BackupStagingStorage{
		PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("300Mi"),
				},
			},
		},
	}
	return backup
}

func buildPhysicalBackupWithS3Storage(bucket, prefix string) func(key types.NamespacedName) *mariadbv1alpha1.PhysicalBackup {
	return func(key types.NamespacedName) *mariadbv1alpha1.PhysicalBackup {
		return getPhysicalBackupWithS3Storage(key, bucket, prefix)
	}
}
