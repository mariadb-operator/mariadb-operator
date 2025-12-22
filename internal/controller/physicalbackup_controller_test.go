package controller

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
			"should reconcile a Job with S3 storage and SSEC",
			"physicalbackup-job-s3-ssec-test",
			applyDecoratorChain(
				buildPhysicalBackupWithS3Storage(testMdbkey, "test-physicalbackup", ""),
				decoratePhysicalBackupWithSSEC,
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

var _ = Describe("PhysicalBackup target", Label("basic"), func() {
	buildTargetFn := func(podIndex *int, err error) targetFn {
		return func(_ context.Context, _ *mariadbv1alpha1.MariaDB, _ logr.Logger) (*int, error) {
			return podIndex, err
		}
	}
	logger := logr.Discard()

	DescribeTable("Getting target Pod index",
		func(
			backup *mariadbv1alpha1.PhysicalBackup,
			mdb *mariadbv1alpha1.MariaDB,
			primaryFn targetFn,
			replicaFn targetFn,
			wantPodIndex *int,
			wantErr bool,
		) {
			podIndex, err := physicalBackupTargetWithFuncs(
				testCtx,
				backup,
				mdb,
				primaryFn,
				replicaFn,
				logger,
			)
			Expect(podIndex).To(BeEquivalentTo(wantPodIndex))

			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
		Entry(
			"standalone",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: nil,
				},
			},
			&mariadbv1alpha1.MariaDB{},
			buildTargetFn(ptr.To(0), nil),
			nil,
			ptr.To(0),
			false,
		),
		Entry(
			"standalone - no target available",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: nil,
				},
			},
			&mariadbv1alpha1.MariaDB{},
			buildTargetFn(nil, errPhysicalBackupNoTargetPodsAvailable),
			nil,
			nil,
			true,
		),
		Entry(
			"HA - default target",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: nil,
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			buildTargetFn(ptr.To(0), nil),
			buildTargetFn(ptr.To(1), nil),
			ptr.To(1),
			false,
		),
		Entry(
			"HA - Replica target",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			nil,
			buildTargetFn(ptr.To(1), nil),
			ptr.To(1),
			false,
		),
		Entry(
			"HA - Replica target, no replicas available",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			nil,
			buildTargetFn(nil, errPhysicalBackupNoTargetPodsAvailable),
			nil,
			true,
		),
		Entry(
			"HA - Replica target, get target error",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			nil,
			buildTargetFn(nil, errors.New("error getting target")),
			nil,
			true,
		),
		Entry(
			"HA - PreferReplica target",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			buildTargetFn(ptr.To(0), nil),
			buildTargetFn(ptr.To(1), nil),
			ptr.To(1),
			false,
		),
		Entry(
			"HA - PreferReplica target, no replicas available",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			buildTargetFn(ptr.To(0), nil),
			buildTargetFn(nil, errPhysicalBackupNoTargetPodsAvailable),
			ptr.To(0),
			false,
		),
		Entry(
			"HA - PreferReplica target, no replicas nor primary available",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			buildTargetFn(nil, errPhysicalBackupNoTargetPodsAvailable),
			buildTargetFn(nil, errPhysicalBackupNoTargetPodsAvailable),
			nil,
			true,
		),
		Entry(
			"HA - PreferReplica target, get target error",
			&mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Target: ptr.To(mariadbv1alpha1.PhysicalBackupTargetPreferReplica),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			buildTargetFn(nil, errors.New("error getting target")),
			buildTargetFn(nil, errors.New("error getting target")),
			nil,
			true,
		),
	)
})
