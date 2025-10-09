package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PhysicalBackup", Label("basic"), func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	DescribeTable("Creating a PhysicalBackup",
		func(
			resourceName string,
			builderFn physicalBackupBuilder,
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
			buildPhysicalBackupWithPVCStorage(testMdbkey),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with Volume storage",
			"physicalbackup-job-volume-test",
			buildPhysicalBackupWithVolumeStorage(testMdbkey),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage",
			"physicalbackup-job-s3-test",
			buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", ""),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage with prefix",
			"physicalbackup-job-s3-prefix-test",
			buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", "mariadb"),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage and bzip2 compression",
			"physicalbackup-job-s3-bzip2-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", ""),
				decoratePhysicalBackupWithBzip2Compression,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage and gzip compression",
			"physicalbackup-job-s3-gzip-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", ""),
				decoratePhysicalBackupWithGzipCompression,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a Job with S3 storage and staging storage",
			"physicalbackup-job-s3-staging-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", ""),
				decoratePhysicalBackupWithStagingStorage,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a scheduled Job with S3 storage",
			"physicalbackup-scheduled-job-s3-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", ""),
				decoratePhysicalBackupWithSchedule,
			),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a VolumeSnapshot",
			"physicalbackup-volumesnapshot-test",
			buildPhysicalBackupWithVolumeSnapshotStorage(testMdbkey),
			testPhysicalBackup,
		),
		Entry(
			"should reconcile a scheduled VolumeSnapshot",
			"physicalbackup-scheduled-volumesnapshot-test",
			applyDecoratorChain(
				buildPhysicalBackupWithVolumeSnapshotStorage(testMdbkey),
				decoratePhysicalBackupWithSchedule,
			),
			testPhysicalBackup,
		),
	)
})
