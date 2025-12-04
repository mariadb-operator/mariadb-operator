package binlog

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/compression"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var uploadBackoff = wait.Backoff{
	Steps:    10,
	Duration: 1 * time.Second,
}

type Uploader struct {
	dataDir    string
	s3Client   *mariadbminio.Client
	client     client.Client
	compressor compression.Compressor
	logger     logr.Logger
}

func NewUploader(dataDir string, s3Client *mariadbminio.Client, client client.Client,
	compressor compression.Compressor, logger logr.Logger) *Uploader {
	return &Uploader{
		dataDir:    dataDir,
		s3Client:   s3Client,
		client:     client,
		compressor: compressor,
		logger:     logger,
	}
}

func (u *Uploader) Upload(ctx context.Context, binlog string, mdb *mariadbv1alpha1.MariaDB,
	pitr *mariadbv1alpha1.PointInTimeRecovery) error {
	startTime := time.Now()
	targetFile, err := u.getTargetFile(binlog, pitr)
	if err != nil {
		return fmt.Errorf("error getting target file: %v", err)
	}
	u.logger.Info(
		"Uploading binary log",
		"binlog", binlog,
		"target-file", targetFile,
		"start-time", startTime,
	)

	if err := u.compressor.Compress(targetFile); err != nil {
		return fmt.Errorf("error compressing binlog: %v", err)
	}

	uploadIsRetriable := func(err error) bool {
		if ctx.Err() != nil {
			return false
		}
		return err != nil
	}
	if err := retry.OnError(uploadBackoff, uploadIsRetriable, func() error {
		return u.s3Client.FPutObjectWithOptions(ctx, targetFile)
	}); err != nil {
		return fmt.Errorf("error uploading binlog %s: %v", binlog, err)
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return u.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.PointInTimeRecovery = &mariadbv1alpha1.PointInTimeRecoveryStatus{
				LastArchivedBinaryLog: &binlog,
			}
		})
	}); err != nil {
		return fmt.Errorf("error patching MariaDB: %v", err)
	}

	u.logger.Info(
		"Binary log uploaded",
		"binlog", binlog,
		"target-file", targetFile,
		"start-time", startTime,
		"total-time", time.Since(startTime),
	)
	return nil
}

func (u *Uploader) getTargetFile(binlog string, pitr *mariadbv1alpha1.PointInTimeRecovery) (string, error) {
	targetFile := binlog

	if pitr.Spec.Compression != "" && pitr.Spec.Compression != mariadbv1alpha1.CompressNone {
		ext, err := pitr.Spec.Compression.Extension()
		if err != nil {
			return "", fmt.Errorf("error getting compression algorithm extension: %v", err)
		}
		targetFile = fmt.Sprintf("%s.%s", targetFile, ext)
	}
	return targetFile, nil
}

func (u *Uploader) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return u.client.Status().Patch(ctx, mariadb, patch)
}
