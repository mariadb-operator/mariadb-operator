package binlog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (u *Uploader) Upload(ctx context.Context, binlog string, suffix string, mdb *mariadbv1alpha1.MariaDB,
	pitr *mariadbv1alpha1.PointInTimeRecovery) error {
	startTime := time.Now()
	targetFile, err := u.getTargetFile(binlog, suffix, pitr)
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
	file, err := u.getBinlogFile(binlog, targetFile, pitr)
	if err != nil {
		return fmt.Errorf("error getting binlog file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting binlog file info: %v", err)
	}

	if err := retry.OnError(uploadBackoff, uploadIsRetriable, func() error {
		return u.s3Client.PutObjectWithOptions(ctx, targetFile, file, fileInfo.Size())
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

func (u *Uploader) getTargetFile(binlog string, suffix string, pitr *mariadbv1alpha1.PointInTimeRecovery) (string, error) {
	name := binlog
	// insert -<suffix> before the first dot in the filename (e.g. mariadb-bin.000001 -> mariadb-bin-<s>.000001)
	if idx := strings.Index(name, "."); idx != -1 {
		name = fmt.Sprintf("%s-%s%s", name[:idx], suffix, name[idx:])
	} else {
		name = fmt.Sprintf("%s-%s", name, suffix)
	}

	if pitr.Spec.Compression != "" && pitr.Spec.Compression != mariadbv1alpha1.CompressNone {
		ext, err := pitr.Spec.Compression.Extension()
		if err != nil {
			return "", fmt.Errorf("error getting compression algorithm extension: %v", err)
		}
		name = fmt.Sprintf("%s.%s", name, ext)
	}
	return name, nil
}

func (u *Uploader) getBinlogFile(binlog string, targetFile string, pitr *mariadbv1alpha1.PointInTimeRecovery) (*os.File, error) {
	if pitr.Spec.Compression == "" || pitr.Spec.Compression == mariadbv1alpha1.CompressNone {
		return os.Open(filepath.Join(u.dataDir, binlog))
	}
	return os.Open(filepath.Join(u.dataDir, targetFile))
}

func (u *Uploader) cleanupCompressedFile(targetFile string, pitr *mariadbv1alpha1.PointInTimeRecovery) error {
	if pitr.Spec.Compression == "" || pitr.Spec.Compression == mariadbv1alpha1.CompressNone {
		return nil
	}
	return os.Remove(filepath.Join(u.dataDir, targetFile))
}
