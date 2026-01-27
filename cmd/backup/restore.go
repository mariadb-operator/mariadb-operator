package backup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v25/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
)

const operatorBinaryPath = "/bin/mariadb-operator"

var (
	physicalBackupDirPath string
	targetTimeRaw         string
	copyBinaryTo          string
)

func init() {
	restoreCommand.Flags().StringVar(&physicalBackupDirPath, "physical-backup-dir-path", "",
		"Directory path where the physical backup has been prepared. Only considered when backup-content-type is Physical.")
	restoreCommand.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the backup target time.")
	restoreCommand.Flags().StringVar(&copyBinaryTo, "copy-binary-to", "",
		"Copy the operator binary to this path for use by subsequent containers in streaming restore.")
}

var restoreCommand = &cobra.Command{
	Use:   "restore",
	Short: "Restore.",
	Long:  `Fetches backup files from multiple storage types and matches the one closest to the target recovery time.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting restore")

		ctx, cancel := newContext()
		defer cancel()

		physicalBackupExists, err := checkPhysicalBackupDir()
		if err != nil {
			logger.Error(err, "error checking physical backup directory")
			os.Exit(1)
		}
		if physicalBackupExists {
			logger.Info("physical backup directory already exists.")
			os.Exit(0)
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

		targetTime, err := getTargetTime()
		if err != nil {
			logger.Error(err, "error getting target time")
			os.Exit(1)
		}
		logger.Info("obtained target time", "time", targetTime.String())

		backupFileNames, err := backupStorage.List(ctx)
		if err != nil {
			logger.Error(err, "error listing backup files")
			os.Exit(1)
		}

		backupTargetFile, err := backupProcessor.GetBackupTargetFile(backupFileNames, targetTime, logger.WithName("target-recovery-time"))
		if err != nil {
			logger.Error(err, "error reading getting target backup")
			os.Exit(1)
		}
		logger.Info("obtained target backup", "file", backupTargetFile)

		// In streaming mode (copyBinaryTo is set), the main container will stream directly from S3.
		// We only need to write the original S3 key to the target file and copy the binary.
		// The actual decompression happens in the main container via: stream | mbstream
		if copyBinaryTo != "" {
			logger.Info("streaming mode: skipping decompression, main container will stream from S3")

			// Write the original S3 key (not local path) so stream command can fetch it
			logger.Info("writing target file with S3 key", "file", targetFilePath, "s3Key", backupTargetFile)
			if err := writeTargetFile(backupTargetFile); err != nil {
				logger.Error(err, "error writing target file", "file", backupTargetFile)
				os.Exit(1)
			}

			logger.Info("copying operator binary for streaming restore", "dest", copyBinaryTo)
			if err := copyOperatorBinary(copyBinaryTo); err != nil {
				logger.Error(err, "error copying operator binary", "dest", copyBinaryTo)
				os.Exit(1)
			}
		} else {
			// Non-streaming mode: decompress to disk, then mariadb-backup will use the local file
			backupCompressor, err := getCompressorWithFile(backupTargetFile, backupProcessor)
			if err != nil {
				logger.Error(err, "error getting backup compressor")
				os.Exit(1)
			}

			logger.Info("restoring target backup", "file", backupTargetFile, "prefix", s3Prefix)
			localFilePath, err := streamingRestore(ctx, backupStorage, backupCompressor, backupTargetFile, path, backupProcessor)
			if err != nil {
				logger.Error(err, "error restoring backup", "file", backupTargetFile)
				os.Exit(1)
			}

			logger.Info("writing target file", "file", targetFilePath, "file-content", localFilePath)
			if err := writeTargetFile(localFilePath); err != nil {
				logger.Error(err, "error writing target file", "file", localFilePath)
				os.Exit(1)
			}
		}
	},
}

func checkPhysicalBackupDir() (bool, error) {
	if backupContentType != string(mariadbv1alpha1.BackupContentTypePhysical) || physicalBackupDirPath == "" {
		return false, nil
	}
	logger.Info("checking existing physical backup directory", "dir-path", physicalBackupDirPath)

	entries, err := os.ReadDir(physicalBackupDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error reading physical backup directory path (%s): %v", physicalBackupDirPath, err)
	}
	return len(entries) > 0, nil
}

func getTargetTime() (time.Time, error) {
	if targetTimeRaw == "" {
		return time.Now(), nil
	}
	return backup.ParseBackupDate(targetTimeRaw)
}

func writeTargetFile(backupTargetFile string) error {
	return os.WriteFile(targetFilePath, []byte(backupTargetFile), 0644)
}

func getCompressorWithFile(fileName string, processor backup.BackupProcessor) (mdbcompression.Compressor, error) {
	calg, err := processor.ParseCompressionAlgorithm(fileName)
	if err != nil {
		return nil, fmt.Errorf("error parsing compression algorithm: %v", err)
	}
	return mdbcompression.NewCompressor(calg, path, processor.GetUncompressedBackupFile, logger)
}

// streamBufferSize is the buffer size used for streaming decompression writes.
const streamBufferSize = 4 * 1024 * 1024 // 4MB

// progressReader wraps an io.Reader and logs progress at specified intervals.
type progressReader struct {
	reader      io.Reader
	totalSize   int64
	bytesRead   atomic.Int64
	lastPercent int
	startTime   time.Time
	fileName    string
}

func newProgressReader(r io.Reader, totalSize int64, fileName string) *progressReader {
	return &progressReader{
		reader:      r,
		totalSize:   totalSize,
		lastPercent: -1,
		startTime:   time.Now(),
		fileName:    fileName,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		newTotal := pr.bytesRead.Add(int64(n))
		pr.logProgress(newTotal)
	}
	if err != nil && err != io.EOF {
		// Log detailed error information
		elapsed := time.Since(pr.startTime)
		bytesRead := pr.bytesRead.Load()
		errInfo := classifyError(err)
		logger.Error(err, "read error during streaming",
			"file", pr.fileName,
			"bytesRead", bytesRead,
			"totalSize", pr.totalSize,
			"percentComplete", float64(bytesRead)/float64(pr.totalSize)*100,
			"elapsed", elapsed.String(),
			"bytesPerSecond", float64(bytesRead)/elapsed.Seconds(),
			"errorDetails", errInfo,
		)
	}
	return n, err
}

func (pr *progressReader) logProgress(bytesRead int64) {
	if pr.totalSize <= 0 {
		return
	}
	percent := int(float64(bytesRead) / float64(pr.totalSize) * 100)
	// Log every 5%
	if percent >= pr.lastPercent+5 {
		pr.lastPercent = (percent / 5) * 5 // Round down to nearest 5%
		elapsed := time.Since(pr.startTime)
		bytesPerSecond := float64(bytesRead) / elapsed.Seconds()
		remainingBytes := pr.totalSize - bytesRead
		etaSeconds := float64(remainingBytes) / bytesPerSecond
		logger.Info("streaming progress",
			"file", pr.fileName,
			"percent", pr.lastPercent,
			"bytesRead", bytesRead,
			"totalSize", pr.totalSize,
			"elapsed", elapsed.Round(time.Second).String(),
			"speedMBps", bytesPerSecond/1024/1024,
			"etaSeconds", int(etaSeconds),
		)
	}
}

func (pr *progressReader) BytesRead() int64 {
	return pr.bytesRead.Load()
}

// classifyError extracts detailed information about an error for debugging.
func classifyError(err error) map[string]interface{} {
	info := make(map[string]interface{})

	// Basic error info
	info["errorType"] = fmt.Sprintf("%T", err)
	info["errorMessage"] = err.Error()

	// Check for standard library errors
	if errors.Is(err, io.EOF) {
		info["category"] = "EOF"
		info["description"] = "End of file reached normally"
	} else if errors.Is(err, io.ErrUnexpectedEOF) {
		info["category"] = "UnexpectedEOF"
		info["description"] = "Stream ended before expected - possible truncation or connection drop"
	} else if errors.Is(err, context.Canceled) {
		info["category"] = "ContextCanceled"
		info["description"] = "Operation was canceled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		info["category"] = "ContextTimeout"
		info["description"] = "Operation timed out"
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		info["isNetworkError"] = true
		info["isTimeout"] = netErr.Timeout()
		if netErr.Timeout() {
			info["category"] = "NetworkTimeout"
			info["description"] = "Network operation timed out"
		}
	}

	// Check for net.OpError (detailed network operation error)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		info["netOp"] = opErr.Op
		info["netNet"] = opErr.Net
		if opErr.Addr != nil {
			info["netAddr"] = opErr.Addr.String()
		}
		if opErr.Source != nil {
			info["netSource"] = opErr.Source.String()
		}
	}

	// Check for URL errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		info["urlOp"] = urlErr.Op
		info["url"] = urlErr.URL
	}

	// Check for syscall errors (OS-level)
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		info["syscallErrno"] = int(syscallErr)
		info["syscallName"] = syscallErr.Error()
		switch syscallErr {
		case syscall.ECONNRESET:
			info["category"] = "ConnectionReset"
			info["description"] = "Connection reset by peer - server closed connection"
		case syscall.ECONNREFUSED:
			info["category"] = "ConnectionRefused"
			info["description"] = "Connection refused by server"
		case syscall.ETIMEDOUT:
			info["category"] = "ConnectionTimeout"
			info["description"] = "Connection timed out at OS level"
		case syscall.EPIPE:
			info["category"] = "BrokenPipe"
			info["description"] = "Broken pipe - write to closed connection"
		case syscall.ECONNABORTED:
			info["category"] = "ConnectionAborted"
			info["description"] = "Connection aborted"
		case syscall.ENETUNREACH:
			info["category"] = "NetworkUnreachable"
			info["description"] = "Network is unreachable"
		case syscall.EHOSTUNREACH:
			info["category"] = "HostUnreachable"
			info["description"] = "Host is unreachable"
		}
	}

	// Check for minio-specific errors
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		info["minioCode"] = minioErr.Code
		info["minioMessage"] = minioErr.Message
		info["minioStatusCode"] = minioErr.StatusCode
		info["minioRequestID"] = minioErr.RequestID
		info["minioBucketName"] = minioErr.BucketName
		info["minioKey"] = minioErr.Key
		info["category"] = "S3Error"
		info["description"] = fmt.Sprintf("S3/Minio error: %s (HTTP %d)", minioErr.Code, minioErr.StatusCode)
	}

	// Check for OS-level file errors
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		info["pathOp"] = pathErr.Op
		info["path"] = pathErr.Path
	}

	// If no category was set, try to infer from error message
	if _, ok := info["category"]; !ok {
		errMsg := err.Error()
		switch {
		case contains(errMsg, "connection reset"):
			info["category"] = "ConnectionReset"
			info["description"] = "Connection was reset (inferred from message)"
		case contains(errMsg, "broken pipe"):
			info["category"] = "BrokenPipe"
			info["description"] = "Broken pipe (inferred from message)"
		case contains(errMsg, "timeout"):
			info["category"] = "Timeout"
			info["description"] = "Timeout occurred (inferred from message)"
		case contains(errMsg, "EOF"):
			info["category"] = "EOFRelated"
			info["description"] = "EOF-related error (inferred from message)"
		default:
			info["category"] = "Unknown"
			info["description"] = "Unknown error type"
		}
	}

	// Get the full error chain
	var chain []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		chain = append(chain, fmt.Sprintf("%T: %s", e, e.Error()))
	}
	if len(chain) > 1 {
		info["errorChain"] = chain
	}

	return info
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func streamingRestore(ctx context.Context, storage backup.BackupStorage,
	compressor mdbcompression.Compressor, backupTargetFile, basePath string,
	processor backup.BackupProcessor) (string, error) {

	streamingStorage, ok1 := storage.(backup.StreamingBackupStorage)
	streamingCompressor, ok2 := compressor.(mdbcompression.StreamingCompressor)

	if !ok1 || !ok2 {
		logger.Info("falling back to legacy two-step restore", "file", backupTargetFile)
		if err := storage.Pull(ctx, backupTargetFile); err != nil {
			return "", err
		}
		return compressor.Decompress(backupTargetFile)
	}

	logger.Info("using streaming restore with resumable downloads", "file", backupTargetFile)

	// Configure resumable reader with retries and exponential backoff
	resumeConfig := mariadbminio.ResumableReaderConfig{
		MaxRetries:     10,                // More retries for large files
		InitialBackoff: 2 * time.Second,   // Start with 2s backoff
		MaxBackoff:     60 * time.Second,  // Max 60s between retries
		Logger:         logger.WithName("resumable-reader"),
	}

	srcReader, size, err := streamingStorage.PullStreamResumable(ctx, backupTargetFile, resumeConfig)
	if err != nil {
		return "", fmt.Errorf("error getting resumable stream from storage: %v", err)
	}
	defer srcReader.Close()

	logger.Info("streaming backup started", "file", backupTargetFile, "compressedSize", size, "compressedSizeGB", float64(size)/1024/1024/1024, "maxRetries", resumeConfig.MaxRetries)

	// Wrap reader with progress tracking
	progressRdr := newProgressReader(srcReader, size, backupTargetFile)

	uncompressedFileName, err := processor.GetUncompressedBackupFile(backupTargetFile)
	if err != nil {
		return "", fmt.Errorf("error getting uncompressed backup file name: %v", err)
	}
	outputFilePath := backup.GetFilePath(basePath, uncompressedFileName)

	// Ensure parent directory exists (backup file may include prefix path like "backups/")
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		return "", fmt.Errorf("error creating output directory: %v", err)
	}

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return "", fmt.Errorf("error creating output file: %v", err)
	}
	defer outputFile.Close()

	startTime := time.Now()
	bufferedWriter := bufio.NewWriterSize(outputFile, streamBufferSize)
	bytesWritten, err := streamingCompressor.DecompressStream(bufferedWriter, progressRdr)
	if err != nil {
		elapsed := time.Since(startTime)
		compressedBytesRead := progressRdr.BytesRead()
		os.Remove(outputFilePath)

		// Detailed error classification
		errInfo := classifyError(err)

		// Detailed error logging
		logger.Error(err, "decompression failed",
			"file", backupTargetFile,
			"compressedBytesRead", compressedBytesRead,
			"compressedTotalSize", size,
			"compressedPercentRead", float64(compressedBytesRead)/float64(size)*100,
			"uncompressedBytesWritten", bytesWritten,
			"elapsed", elapsed.String(),
			"errorCategory", errInfo["category"],
			"errorDescription", errInfo["description"],
			"errorType", errInfo["errorType"],
			"errorDetails", errInfo,
		)

		return "", fmt.Errorf("error decompressing stream after reading %d/%d bytes (%.1f%%) in %s [%s: %s]: %w",
			compressedBytesRead, size, float64(compressedBytesRead)/float64(size)*100, elapsed,
			errInfo["category"], errInfo["description"], err)
	}

	if err := bufferedWriter.Flush(); err != nil {
		os.Remove(outputFilePath)
		return "", fmt.Errorf("error flushing output buffer: %v", err)
	}

	if err := outputFile.Sync(); err != nil {
		os.Remove(outputFilePath)
		return "", fmt.Errorf("error syncing output file to disk: %v", err)
	}

	elapsed := time.Since(startTime)
	compressedBytesRead := progressRdr.BytesRead()

	// Get total retries if using resumable reader
	totalRetries := 0
	if rr, ok := srcReader.(*mariadbminio.ResumableReader); ok {
		totalRetries = rr.TotalRetries()
	}

	logger.Info("streaming restore completed",
		"file", backupTargetFile,
		"compressedBytesRead", compressedBytesRead,
		"uncompressedBytesWritten", bytesWritten,
		"compressionRatio", float64(bytesWritten)/float64(compressedBytesRead),
		"elapsed", elapsed.String(),
		"speedMBps", float64(compressedBytesRead)/elapsed.Seconds()/1024/1024,
		"totalRetries", totalRetries,
		"outputFile", outputFilePath,
	)
	return outputFilePath, nil
}

func copyOperatorBinary(destPath string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating destination directory: %v", err)
	}

	src, err := os.Open(operatorBinaryPath)
	if err != nil {
		return fmt.Errorf("error opening source binary: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("error creating destination binary: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copying binary: %v", err)
	}

	return dst.Sync()
}
