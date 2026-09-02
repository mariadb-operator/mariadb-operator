package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(streamCommand)
}

var streamCommand = &cobra.Command{
	Use:   "stream",
	Short: "Stream a backup file to stdout.",
	Long:  `Streams a backup file from storage to stdout, decompressing on the fly. Designed to be piped to mbstream.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting stream")

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

		logger.Info("reading target file", "file", targetFilePath)
		backupTargetFile, err := readTargetFile()
		if err != nil {
			logger.Error(err, "error reading target file", "file", targetFilePath)
			os.Exit(1)
		}
		backupTargetFile = strings.TrimSpace(backupTargetFile)
		logger.Info("obtained target backup", "file", backupTargetFile)

		if err := streamBackup(ctx, backupStorage, backupProcessor, backupTargetFile); err != nil {
			logger.Error(err, "error streaming backup", "file", backupTargetFile)
			os.Exit(1)
		}
		logger.Info("stream completed successfully")
	},
}

func streamBackup(ctx context.Context, storage backup.BackupStorage, processor backup.BackupProcessor,
	backupTargetFile string) error {
	if err := backup.ValidateFilename(backupTargetFile); err != nil {
		return fmt.Errorf("invalid backup filename: %w", err)
	}

	blobStorage, ok := storage.(*backup.BlobBackupStorage)
	if !ok {
		return fmt.Errorf("streaming is only supported for blob storage (S3/Azure), got %T", storage)
	}

	calg, err := processor.ParseCompressionAlgorithm(backupTargetFile)
	if err != nil {
		return fmt.Errorf("error parsing compression algorithm: %v", err)
	}
	compressor, err := mdbcompression.NewCompressor(calg)
	if err != nil {
		return fmt.Errorf("error creating compressor: %v", err)
	}

	config := mariadbminio.DefaultResumableReaderConfig(logger.WithName("resumable-reader"))
	reader, totalSize, err := blobStorage.PullStreamResumable(ctx, backupTargetFile, config)
	if err != nil {
		return fmt.Errorf("error getting stream reader: %v", err)
	}
	defer reader.Close()

	var progressReader io.Reader = reader
	if totalSize > 0 {
		progressReader = newProgressReader(reader, totalSize, logger)
	}

	if err := compressor.Decompress(ctx, os.Stdout, progressReader); err != nil {
		errType := classifyError(err)
		return fmt.Errorf("stream decompression failed (%s): %w", errType, err)
	}
	return nil
}

// progressReader wraps an io.Reader with progress tracking and logging.
type progressReader struct {
	reader      io.Reader
	totalSize   int64
	bytesRead   atomic.Int64
	lastPercent atomic.Int32
	startTime   time.Time
	logger      interface{ Info(string, ...interface{}) }
}

func newProgressReader(reader io.Reader, totalSize int64, l interface{ Info(string, ...interface{}) }) *progressReader {
	return &progressReader{
		reader:    reader,
		totalSize: totalSize,
		startTime: time.Now(),
		logger:    l,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		newTotal := pr.bytesRead.Add(int64(n))
		pr.logProgress(newTotal)
	}
	return n, err
}

func (pr *progressReader) logProgress(bytesRead int64) {
	if pr.totalSize <= 0 {
		return
	}
	percent := int32(float64(bytesRead) / float64(pr.totalSize) * 100)
	lastPct := pr.lastPercent.Load()
	// Log every 5%.
	rounded := (percent / 5) * 5
	lastRounded := (lastPct / 5) * 5
	if rounded > lastRounded && pr.lastPercent.CompareAndSwap(lastPct, percent) {
		elapsed := time.Since(pr.startTime)
		speed := float64(bytesRead) / elapsed.Seconds()
		remaining := time.Duration(float64(pr.totalSize-bytesRead)/speed) * time.Second

		pr.logger.Info("Stream progress",
			"percent", fmt.Sprintf("%d%%", percent),
			"bytesRead", bytesRead,
			"totalSize", pr.totalSize,
			"speed", fmt.Sprintf("%.1f MB/s", speed/1024/1024),
			"eta", remaining.Truncate(time.Second),
		)
	}
}

// BytesRead returns the total number of bytes read so far.
func (pr *progressReader) BytesRead() int64 {
	return pr.bytesRead.Load()
}

// classifyError returns a human-readable error classification for logging.
func classifyError(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, io.EOF) {
		return "eof"
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return "unexpected-eof"
	}
	if errors.Is(err, context.Canceled) {
		return "context-canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context-deadline"
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "network-timeout"
		}
		return "network-error"
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Sprintf("network-op-error(%s)", opErr.Op)
	}

	var sysErr *os.SyscallError
	if errors.As(err, &sysErr) {
		return fmt.Sprintf("syscall-error(%s)", sysErr.Syscall)
	}

	if errors.Is(err, syscall.ECONNRESET) {
		return "connection-reset"
	}
	if errors.Is(err, syscall.EPIPE) {
		return "broken-pipe"
	}
	if errors.Is(err, syscall.ENETUNREACH) {
		return "network-unreachable"
	}

	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		return fmt.Sprintf("s3-error(%d:%s)", minioErr.StatusCode, minioErr.Code)
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Sprintf("path-error(%s)", pathErr.Op)
	}

	return "unknown"
}
