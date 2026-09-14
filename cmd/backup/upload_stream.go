package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/xbstream"
	"github.com/spf13/cobra"
)

const (
	defaultStreamUploadPartSize = 512 * 1024 * 1024
	maxCapturedMetadataBytes    = 1024 * 1024
)

var streamUploadPartSize uint64

func init() {
	uploadStreamCommand.Flags().Uint64Var(&streamUploadPartSize, "stream-upload-part-size",
		defaultStreamUploadPartSize,
		"S3 multipart part size in bytes for streaming uploads with unknown total size.")
	RootCmd.AddCommand(uploadStreamCommand)
}

var uploadStreamCommand = &cobra.Command{
	Use:   "upload-stream",
	Short: "Upload a backup stream from stdin.",
	Long:  `Uploads a backup stream from stdin to object storage, compressing on the fly.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting streaming upload")

		ctx, cancel := newContext()
		defer cancel()

		if !s3 {
			logger.Error(errors.New("unsupported storage"), "streaming upload is only supported for S3 storage")
			os.Exit(1)
		}

		backupProcessor, err := getBackupProcessor()
		if err != nil {
			logger.Error(err, "error getting backup processor")
			os.Exit(1)
		}
		backupStorage, err := getBackupStorage(backupProcessor)
		if err != nil {
			logger.Error(err, "error getting backup storage")
			os.Exit(1)
		}
		compressor, err := getStreamCompressor()
		if err != nil {
			logger.Error(err, "error getting stream compressor")
			os.Exit(1)
		}

		logger.Info("reading target file", "file", targetFilePath)
		backupTargetFile, err := readTargetFile()
		if err != nil {
			logger.Error(err, "error reading target file", "file", targetFilePath)
			os.Exit(1)
		}
		backupTargetFile = strings.TrimSpace(backupTargetFile)
		if err := backup.ValidateFilename(backupTargetFile); err != nil {
			logger.Error(err, "invalid backup target file", "file", backupTargetFile)
			os.Exit(1)
		}
		logger.Info("obtained target backup", "file", backupTargetFile)

		var metadataCapture *xbstream.MetadataCapture
		uploadSource := io.Reader(os.Stdin)
		if shouldCapturePhysicalBackupMeta() {
			metadataCapture = xbstream.NewMetadataCapture(
				[]string{replication.BinlogFileName, replication.LegacyBinlogFileName},
				maxCapturedMetadataBytes,
			)
			uploadSource = metadataCapture.WrapReader(uploadSource)
		}

		logger.Info("streaming target backup", "file", backupTargetFile, "part-size", streamUploadPartSize)
		if err := uploadCompressedStream(ctx, backupStorage, backupTargetFile, uploadSource, compressor); err != nil {
			logger.Error(err, "error streaming target backup", "file", backupTargetFile)
			os.Exit(1)
		}

		if metadataCapture != nil {
			metaBytes, metaFile, ok := metadataCapture.Metadata()
			if !ok {
				logger.Error(errors.New("metadata not found"), "physical backup metadata was not found in stream")
				os.Exit(1)
			}
			logger.Info("captured physical backup metadata", "file", metaFile)
			if err := handleBackupMetaBytes(ctx, logger.WithName("backup-meta"), metaBytes); err != nil {
				logger.Error(err, "error handling physical backup meta")
				os.Exit(1)
			}
		}

		if err := cleanupOldBackups(ctx, backupStorage, backupProcessor, logger.WithName("backup-cleanup")); err != nil {
			logger.Error(err, "error cleaning up old backups")
			os.Exit(1)
		}
		logger.Info("streaming upload completed successfully")
	},
}

func uploadCompressedStream(ctx context.Context, backupStorage backup.BackupStorage, backupTargetFile string,
	source io.Reader, compressor mdbcompression.Compressor) error {
	pipeReader, pipeWriter := io.Pipe()
	compressErrCh := make(chan error, 1)

	go func() {
		err := compressor.Compress(ctx, pipeWriter, source)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			compressErrCh <- err
			return
		}
		compressErrCh <- pipeWriter.Close()
	}()

	uploadErr := backupStorage.PushStream(ctx, backupTargetFile, pipeReader, streamUploadPartSize)
	_ = pipeReader.Close()
	compressErr := <-compressErrCh
	if uploadErr != nil {
		return uploadErr
	}
	if compressErr != nil {
		return compressErr
	}
	return nil
}

func getStreamCompressor() (mdbcompression.Compressor, error) {
	calg := mariadbv1alpha1.CompressAlgorithm(compression)
	if err := calg.Validate(); err != nil {
		return nil, fmt.Errorf("compression algorithm not supported: %v", err)
	}
	return mdbcompression.NewCompressor(calg)
}

func cleanupOldBackups(ctx context.Context, backupStorage backup.BackupStorage, backupProcessor backup.BackupProcessor,
	cleanupLogger logr.Logger) error {
	cleanupLogger.Info("cleaning up old backups")
	backupNames, err := backupStorage.List(ctx)
	if err != nil {
		return fmt.Errorf("error listing backup files: %v", err)
	}
	oldBackups := backupProcessor.GetOldBackupFiles(backupNames, maxRetention, cleanupLogger)
	cleanupLogger.Info("old backups to delete", "backups", len(oldBackups))
	for _, backupName := range oldBackups {
		cleanupLogger.V(1).Info("deleting old backup", "backup", backupName)
		if err := backupStorage.Delete(ctx, backupName); err != nil {
			cleanupLogger.Error(err, "error removing old backup", "backup", backupName)
		}
	}
	return nil
}

func shouldCapturePhysicalBackupMeta() bool {
	return backupContentType == string(mariadbv1alpha1.BackupContentTypePhysical) &&
		physicalBackupMeta &&
		physicalBackupName != "" &&
		physicalBackupNamespace != ""
}
