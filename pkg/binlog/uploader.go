package binlog

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
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
	fileManager *filemanager.FileManager
	s3Client    *mariadbminio.Client
	client      client.Client
	logger      logr.Logger
}

func NewUploader(fileManager *filemanager.FileManager, s3Client *mariadbminio.Client,
	client client.Client, logger logr.Logger) *Uploader {
	return &Uploader{
		fileManager: fileManager,
		s3Client:    s3Client,
		client:      client,
		logger:      logger,
	}
}

func (u *Uploader) Upload(ctx context.Context, binlog string, mdb *mariadbv1alpha1.MariaDB,
	pitr *mariadbv1alpha1.PointInTimeRecovery) error {
	startTime := time.Now()
	targetFile := u.fileManager.StateFilePath(binlog)
	u.logger.Info(
		"Uploading binary log",
		"binlog", binlog,
		"target-file", targetFile,
		"start-time", startTime,
	)

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

func (u *Uploader) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return u.client.Status().Patch(ctx, mariadb, patch)
}
