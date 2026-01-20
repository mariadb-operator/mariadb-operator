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
	binlogFileName := filepath.Join(u.dataDir, binlog)
	meta, err := GetBinlogMetadata(binlogFileName, u.logger)
	if err != nil {
		return fmt.Errorf("error getting binary log %s metadata: %v", binlog, err)
	}
	objectName, err := getObjectName(binlog, meta, pitr)
	if err != nil {
		return fmt.Errorf("error getting object name: %v", err)
	}
	binlogLogger := u.logger.WithValues(
		"binlog", binlog,
		"object", objectName,
	)

	exists, err := u.s3Client.Exists(ctx, objectName)
	if err != nil {
		return fmt.Errorf("error determining if binary log exists: %v", err)
	}
	if exists {
		binlogLogger.V(1).Info("Binary log already exists. Skipping...")
		return nil
	}

	startTime := time.Now()
	binlogLogger = binlogLogger.WithValues("start-time", startTime.Format(time.RFC3339))
	binlogLogger.Info("Uploading binary log")

	binlogFile, err := os.Open(binlogFileName)
	if err != nil {
		return fmt.Errorf("error opening binlog file %s: %v", binlogFileName, err)
	}
	defer binlogFile.Close()

	if pitr.Spec.Compression != "" && pitr.Spec.Compression != mariadbv1alpha1.CompressNone {
		binlogLogger.Info("Compressing binary log")
	}
	// Temporary file to be used for compression. This is to avoid loading the binlog in memory.
	tmpFile, err := os.CreateTemp(u.dataDir, binlog+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %v", u.dataDir, err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if err := u.compressor.Compress(tmpFile, binlogFile); err != nil {
		return fmt.Errorf("error compressing binlog: %v", err)
	}
	tmpStat, err := tmpFile.Stat()
	if err != nil {
		return fmt.Errorf("error stat temp file %s: %v", tmpFile.Name(), err)
	}

	uploadIsRetriable := func(err error) bool {
		if ctx.Err() != nil {
			return false
		}
		return err != nil
	}
	if err := retry.OnError(uploadBackoff, uploadIsRetriable, func() error {
		// rewind to start for each attempt
		if _, err := tmpFile.Seek(0, 0); err != nil {
			return fmt.Errorf("error seeking before upload: %v", err)
		}
		return u.s3Client.PutObjectWithOptions(ctx, objectName, tmpFile, tmpStat.Size())
	}); err != nil {
		return fmt.Errorf("error uploading binlog %s: %v", binlog, err)
	}

	binlogLogger.Info("Binary log uploaded", "total-time", time.Since(startTime).String())
	return nil
}

func getObjectName(binlog string, meta *BinlogMetadata, pitr *mariadbv1alpha1.PointInTimeRecovery) (string, error) {
	name := binlog
	if pitr.Spec.Compression != "" && pitr.Spec.Compression != mariadbv1alpha1.CompressNone {
		ext, err := pitr.Spec.Compression.Extension()
		if err != nil {
			return "", fmt.Errorf("error getting compression algorithm extension: %v", err)
		}
		name = fmt.Sprintf("%s.%s", name, ext)
	}
	return fmt.Sprintf("server-%d/%s", meta.ServerId, name), nil
}
