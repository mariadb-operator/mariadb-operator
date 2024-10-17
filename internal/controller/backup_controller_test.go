package controller

import (
	"reflect"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Backup", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	DescribeTable("Creating a Backup",
		func(
			resourceName string,
			builderFn func(types.NamespacedName) *mariadbv1alpha1.Backup,
			testFn func(*mariadbv1alpha1.Backup),
		) {
			key := types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}
			backup := builderFn(key)
			testFn(backup)
		},
		Entry("should reconcile a Job with PVC storage",
			"backup-pvc-test",
			getBackupWithPVCStorage,
			testBackup,
		),
		Entry("should reconcile a Job with Volume storage",
			"backup-volume-test",
			getBackupWithVolumeStorage,
			testBackup,
		),
		Entry("should reconcile a Job with Volume storage and gzip compression",
			"backup-volume-gzip-test",
			applyDecoratorChain(
				getBackupWithVolumeStorage,
				decorateBackupWithGzipCompression,
			),
			testBackup,
		),
		Entry("should reconcile a Job with S3 storage",
			"backup-s3-test",
			buildBackupWithS3Storage("test-backup", ""),
			testS3Backup,
		),
		Entry("should reconcile a Job with S3 storage and bzip2 compression",
			"backup-s3-bzip2-test",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", ""),
				decorateBackupWithBzip2Compression,
			),
			testS3Backup,
		),
		Entry("should reconcile a Job with S3 storage and staging storage",
			"backup-s3-staging-test",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", ""),
				decorateBackupWithStagingStorage,
			),
			testS3Backup,
		),
		Entry("should reconcile a Job with S3 storage with prefix",
			"backup-s3-test-prefix",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", "mariadb"),
				decorateBackupWithNoneCompression,
			),
			testS3Backup,
		),
		Entry("should reconcile a CronJob with PVC storage",
			"backup-pvc-scheduled-test",
			getBackupWithPVCStorage,
			testBackup,
		),
		Entry("should reconcile a CronJob with Volume storage",
			"backup-volume-scheduled-test",
			getBackupWithPVCStorage,
			testBackup,
		),
		Entry("should reconcile a CronJob with S3 storage",
			"backup-s3-scheduled-test",
			buildBackupWithS3Storage("test-backup", ""),
			testS3Backup,
		),
		Entry(
			"should reconcile a CronJob with S3 storage with prefix",
			"backup-s3-scheduled-test-prefix",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", "mariadb"),
				decorateBackupWithSchedule,
			),
			testS3Backup,
		),
		Entry(
			"should reconcile a CronJob with S3 storage and staging storage",
			"backup-s3-scheduled-staging",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", ""),
				decorateBackupWithSchedule,
				decorateBackupWithStagingStorage,
			),
			testS3Backup,
		),
		Entry("should reconcile a CronJob with PVC storage and history limits",
			"backup-pvc-scheduled-with-limits-test",
			applyDecoratorChain(
				getBackupWithPVCStorage,
				decorateBackupWithSchedule,
				decorateBackupWithHistoryLimits,
			),
			testBackup,
		),
		Entry("should reconcile a CronJob with Volume storage and history limits",
			"backup-volume-scheduled-with-limits-test",
			applyDecoratorChain(
				getBackupWithVolumeStorage,
				decorateBackupWithSchedule,
				decorateBackupWithHistoryLimits,
			),
			testBackup,
		),
		Entry("should reconcile a CronJob with S3 storage and history limits",
			"backup-s3-scheduled-with-limits-test",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", ""),
				decorateBackupWithSchedule,
				decorateBackupWithHistoryLimits,
			),
			testS3Backup,
		),
		Entry(
			"should reconcile a CronJob with S3 storage with prefix and history limits",
			"backup-s3-scheduled-with-limits-test-prefix",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", "mariadb"),
				decorateBackupWithSchedule,
				decorateBackupWithHistoryLimits,
			),
			testS3Backup,
		),
		Entry("should reconcile a CronJob with PVC storage and time zone setting",
			"backup-pvc-scheduled-with-tz-test",
			applyDecoratorChain(
				getBackupWithPVCStorage,
				decorateBackupWithSchedule,
				decorateBackupWithTimeZone,
			),
			testBackup,
		),
		Entry("should reconcile a CronJob with Volume storage and time zone setting",
			"backup-volume-scheduled-with-tz-test",
			applyDecoratorChain(
				getBackupWithVolumeStorage,
				decorateBackupWithSchedule,
				decorateBackupWithTimeZone,
			),
			testBackup,
		),
		Entry("should reconcile a CronJob with S3 storage and time zone setting",
			"backup-s3-scheduled-with-tz-test",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", ""),
				decorateBackupWithSchedule,
				decorateBackupWithTimeZone,
			),
			testS3Backup,
		),
		Entry(
			"should reconcile a CronJob with S3 storage with prefix and time zone setting",
			"backup-s3-scheduled-with-tz-test-prefix",
			applyDecoratorChain(
				buildBackupWithS3Storage("test-backup", "mariadb"),
				decorateBackupWithSchedule,
				decorateBackupWithTimeZone,
			),
			testS3Backup,
		),
	)
})

func testBackup(backup *mariadbv1alpha1.Backup) {
	key := client.ObjectKeyFromObject(backup)

	By("Creating Backup")
	Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
	})

	By("Expecting to create a ServiceAccount eventually")
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(testCtx, key, backup)).To(Succeed())
		var svcAcc corev1.ServiceAccount
		key := backup.Spec.JobPodTemplate.ServiceAccountKey(backup.ObjectMeta)
		g.Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())

		g.Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
		g.Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
		g.Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
		g.Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	if backup.Spec.Schedule != nil {
		testBackupCronJob(backup)
	} else {
		testBackupJob(backup)
	}
}

func testBackupJob(backup *mariadbv1alpha1.Backup) {
	key := client.ObjectKeyFromObject(backup)

	var job batchv1.Job
	By("Expecting to create a Job eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, &job); err != nil {
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

	By("Expecting Job to have metadata")
	Expect(job.ObjectMeta.Labels).NotTo(BeNil())
	Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
	Expect(job.ObjectMeta.Annotations).NotTo(BeNil())
	Expect(job.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))

	By("Expecting Backup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		return backup.IsComplete()
	}, testTimeout, testInterval).Should(BeTrue())
}

func testBackupCronJob(backup *mariadbv1alpha1.Backup) {
	By("Expecting to create a CronJob eventually")
	Eventually(func() bool {
		var cronJob batchv1.CronJob
		err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(backup), &cronJob)
		if err != nil {
			return false
		}
		isScheduleCorrect := cronJob.Spec.Schedule == backup.Spec.Schedule.Cron

		if backup.Spec.SuccessfulJobsHistoryLimit == nil {
			// Kubernetes sets a default of 3 when no limit is specified.
			backup.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](3)
		}

		if backup.Spec.FailedJobsHistoryLimit == nil {
			// Kubernetes sets a default of 1 when no limit is specified.
			backup.Spec.FailedJobsHistoryLimit = ptr.To[int32](1)
		}

		return isScheduleCorrect && assertBackupCronJobTemplateSpecsEqual(cronJob, backup)
	}, testHighTimeout, testInterval).Should(BeTrue())

	patch := client.MergeFrom(backup.DeepCopy())
	backup.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](7)
	backup.Spec.FailedJobsHistoryLimit = ptr.To[int32](7)
	backup.Spec.TimeZone = ptr.To[string]("Europe/Madrid")
	By("Updating a CronJob's history limits and time zone")
	Expect(k8sClient.Patch(testCtx, backup, patch)).To(Succeed())

	By("Expecting to update the CronJob history limits and time zone eventually")
	Eventually(func() bool {
		var cronJob batchv1.CronJob
		if k8sClient.Get(testCtx, client.ObjectKeyFromObject(backup), &cronJob) != nil {
			return false
		}
		return assertBackupCronJobTemplateSpecsEqual(cronJob, backup)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func testS3Backup(backup *mariadbv1alpha1.Backup) {
	By("Creating Backup with S3 storage")
	Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, backup)).To(Succeed())
	})

	if backup.Spec.Schedule != nil {
		testBackupCronJob(backup)
	} else {
		By("Expecting Backup to complete eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(backup), backup); err != nil {
				return false
			}
			return backup.IsComplete()
		}, testTimeout, testInterval).Should(BeTrue())
	}
}

func decorateBackupWithSchedule(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
	backup.Spec.Schedule = &mariadbv1alpha1.Schedule{Cron: "*/5 * * * *"}
	return backup
}

func decorateBackupWithHistoryLimits(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
	backup.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](5)
	backup.Spec.FailedJobsHistoryLimit = ptr.To[int32](5)
	return backup
}

func decorateBackupWithTimeZone(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
	backup.Spec.TimeZone = ptr.To[string]("Europe/Sofia")
	return backup
}

func decorateBackupWithNoneCompression(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
	backup.Spec.Compression = mariadbv1alpha1.CompressNone
	return backup
}

func decorateBackupWithGzipCompression(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
	backup.Spec.Compression = mariadbv1alpha1.CompressGzip
	return backup
}

func decorateBackupWithBzip2Compression(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
	backup.Spec.Compression = mariadbv1alpha1.CompressBzip2
	return backup
}

func decorateBackupWithStagingStorage(backup *mariadbv1alpha1.Backup) *mariadbv1alpha1.Backup {
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

func buildBackupWithS3Storage(bucket, prefix string) func(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return func(key types.NamespacedName) *mariadbv1alpha1.Backup {
		return getBackupWithS3Storage(key, bucket, prefix)
	}
}

func assertBackupCronJobTemplateSpecsEqual(cronJob batchv1.CronJob, backup *mariadbv1alpha1.Backup) bool {
	isSuccessfulJobHistoryLimitCorrect :=
		reflect.DeepEqual(cronJob.Spec.SuccessfulJobsHistoryLimit, backup.Spec.SuccessfulJobsHistoryLimit)
	isFailedJobHistoryLimitCorrect :=
		reflect.DeepEqual(cronJob.Spec.FailedJobsHistoryLimit, backup.Spec.FailedJobsHistoryLimit)
	isTimeZoneCorrect := reflect.DeepEqual(cronJob.Spec.TimeZone, backup.Spec.TimeZone)
	return isSuccessfulJobHistoryLimitCorrect && isFailedJobHistoryLimitCorrect && isTimeZoneCorrect
}
