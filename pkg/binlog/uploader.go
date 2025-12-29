package binlog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/compression"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

var uploadBackoff = wait.Backoff{
	Steps:    10,
	Duration: 1 * time.Second,
}

type Uploader struct {
	dataDir    string
	s3Client   *mariadbminio.Client
	compressor compression.Compressor
	logger     logr.Logger
}

func NewUploader(dataDir string, s3Client *mariadbminio.Client, compressor compression.Compressor,
	logger logr.Logger) *Uploader {
	return &Uploader{
		dataDir:    dataDir,
		s3Client:   s3Client,
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
		"start-time", startTime.Format(time.RFC3339),
	)

	if err := u.compressor.Compress(binlog, compression.WithCompressedFilename(targetFile)); err != nil {
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

	if err := u.cleanupCompressedFile(targetFile, pitr); err != nil {
		return fmt.Errorf("error cleaning up compressed file: %v", err)
	}

	u.logger.Info(
		"Binary log uploaded",
		"binlog", binlog,
		"target-file", targetFile,
		"start-time", startTime.Format(time.RFC3339),
		"total-time", time.Since(startTime).String(),
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

func (u *Uploader) cleanupCompressedFile(targetFile string, pitr *mariadbv1alpha1.PointInTimeRecovery) error {
	if pitr.Spec.Compression == "" || pitr.Spec.Compression == mariadbv1alpha1.CompressNone {
		return nil
	}
	return os.Remove(filepath.Join(u.dataDir, targetFile))
}
