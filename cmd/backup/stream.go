package backup

import (
	"fmt"
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v25/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(streamCommand)
}

var streamCommand = &cobra.Command{
	Use:   "stream",
	Short: "Stream backup to stdout.",
	Long: `Streams a backup file from storage directly to stdout with decompression.
This command is designed to be piped into mbstream for physical backup restoration,
eliminating the need for intermediate files on disk.

Example:
  mariadb-operator backup stream --s3 --s3-bucket mybucket | mbstream -x -C /backup/full`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting backup stream")

		ctx, cancel := newContext()
		defer cancel()

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

		// Get streaming storage interface
		streamingStorage, ok := backupStorage.(backup.StreamingBackupStorage)
		if !ok {
			logger.Error(nil, "storage does not support streaming")
			os.Exit(1)
		}

		logger.Info("reading target file", "file", targetFilePath)
		backupTargetFile, err := readTargetFile()
		if err != nil {
			logger.Error(err, "error reading target file", "file", targetFilePath)
			os.Exit(1)
		}
		logger.Info("streaming backup", "file", backupTargetFile)

		// Get compressor for decompression
		backupCompressor, err := getCompressorWithFile(backupTargetFile, backupProcessor)
		if err != nil {
			logger.Error(err, "error getting backup compressor")
			os.Exit(1)
		}

		streamingCompressor, ok := backupCompressor.(mdbcompression.StreamingCompressor)
		if !ok {
			logger.Error(nil, "compressor does not support streaming")
			os.Exit(1)
		}

		// Configure resumable reader with retries and exponential backoff for large files
		resumeConfig := mariadbminio.ResumableReaderConfig{
			MaxRetries:     10,               // More retries for large files
			InitialBackoff: 2 * time.Second,  // Start with 2s backoff
			MaxBackoff:     60 * time.Second, // Max 60s between retries
			Logger:         logger.WithName("resumable-reader"),
		}

		// Get resumable stream from storage
		srcReader, size, err := streamingStorage.PullStreamResumable(ctx, backupTargetFile, resumeConfig)
		if err != nil {
			logger.Error(err, "error getting resumable stream from storage", "file", backupTargetFile)
			os.Exit(1)
		}
		defer srcReader.Close()

		logger.Info("streaming backup to stdout", "file", backupTargetFile, "compressedSize", size, "maxRetries", resumeConfig.MaxRetries)

		// Wrap reader with progress tracking
		progressRdr := newProgressReader(srcReader, size, backupTargetFile)

		// Stream decompress directly to stdout
		bytesWritten, err := streamingCompressor.DecompressStream(os.Stdout, progressRdr)
		if err != nil {
			// Get retry count for error logging
			totalRetries := 0
			if rr, ok := srcReader.(*mariadbminio.ResumableReader); ok {
				totalRetries = rr.TotalRetries()
			}
			errInfo := classifyError(err)
			logger.Error(err, "error streaming backup",
				"file", backupTargetFile,
				"compressedBytesRead", progressRdr.BytesRead(),
				"compressedTotalSize", size,
				"totalRetries", totalRetries,
				"errorCategory", errInfo["category"],
				"errorDescription", errInfo["description"],
			)
			os.Exit(1)
		}

		// Get retry count for completion logging
		totalRetries := 0
		if rr, ok := srcReader.(*mariadbminio.ResumableReader); ok {
			totalRetries = rr.TotalRetries()
		}

		logger.Info("streaming completed",
			"bytesWritten", bytesWritten,
			"compressedBytesRead", progressRdr.BytesRead(),
			"totalRetries", totalRetries,
		)
	},
}
