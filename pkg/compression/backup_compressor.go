package compression

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

type BackupCompressor interface {
	// Compress compresses fileName in place and returns the final, post-compression file name
	// (e.g. "backup.<ts>.sql" -> "backup.<ts>.sql.gz"). For NopBackupCompressor the input name
	// is returned unchanged. The operation is idempotent: if the post-compression file already
	// exists from a previous run, no work is performed and the existing name is returned.
	Compress(fileName string) (string, error)
	Decompress(fileName string) (string, error)
}

type GetBackupUncompressedFilenameFn func(compressedFilename string) (string, error)

func NewBackupCompressor(calg mariadbv1alpha1.CompressAlgorithm, basePath string,
	getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) (BackupCompressor, error) {
	switch calg {
	case mariadbv1alpha1.CompressNone:
		return NewNopBackupCompressor(basePath, getUncompressedFilename, logger.WithName("nop-compressor")), nil
	case mariadbv1alpha1.CompressGzip:
		return NewGzipBackupCompressor(basePath, getUncompressedFilename, logger.WithName("gzip-compressor")), nil
	case mariadbv1alpha1.CompressBzip2:
		return NewBzip2BackupCompressor(basePath, getUncompressedFilename, logger.WithName("bzip2-compressor")), nil
	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %v", calg)
	}
}

type NopBackupCompressor struct {
	basePath string
}

func NewNopBackupCompressor(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) BackupCompressor {
	return &NopBackupCompressor{
		basePath: basePath,
	}
}

func (c *NopBackupCompressor) Compress(fileName string) (string, error) {
	return fileName, nil
}

func (c *NopBackupCompressor) Decompress(fileName string) (string, error) {
	return getFilePath(c.basePath, fileName), nil
}

type GzipBackupCompressor struct {
	compressor              *GzipCompressor
	basePath                string
	getUncompressedFilename GetBackupUncompressedFilenameFn
	logger                  logr.Logger
}

func NewGzipBackupCompressor(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn,
	logger logr.Logger) BackupCompressor {
	return &GzipBackupCompressor{
		compressor:              &GzipCompressor{},
		basePath:                basePath,
		getUncompressedFilename: getUncompressedFilename,
		logger:                  logger,
	}
}

func (c *GzipBackupCompressor) Compress(fileName string) (string, error) {
	return compressFile(c.basePath, fileName, "gz", c.logger, c.compressor)
}

func (c *GzipBackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, c.compressor)
}

type Bzip2BackupCompressor struct {
	compressor              *Bzip2Compressor
	basePath                string
	getUncompressedFilename GetBackupUncompressedFilenameFn
	logger                  logr.Logger
}

func NewBzip2BackupCompressor(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn,
	logger logr.Logger) BackupCompressor {
	return &Bzip2BackupCompressor{
		compressor:              &Bzip2Compressor{},
		basePath:                basePath,
		getUncompressedFilename: getUncompressedFilename,
		logger:                  logger,
	}
}

func (c *Bzip2BackupCompressor) Compress(fileName string) (string, error) {
	return compressFile(c.basePath, fileName, "bz2", c.logger, c.compressor)
}

func (c *Bzip2BackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, c.compressor)
}

// compressFile compresses path/fileName into path/fileName.<ext> and removes the plain source on success.
//
// Idempotency: if the compressed output already exists (e.g. a previous container attempt completed
// the compress step but crashed before push), the function returns the existing output name without
// re-compressing. This prevents nested compression layers when the operator-backup container is
// restarted by the kubelet under RestartPolicy: OnFailure.
func compressFile(path, fileName, ext string, logger logr.Logger, compressor Compressor) (string, error) {
	plainFilePath := getFilePath(path, fileName)
	compressedFileName := fileName + "." + ext
	compressedFilePath := plainFilePath + "." + ext

	if _, err := os.Stat(compressedFilePath); err == nil {
		logger.Info("compressed file already exists, skipping compression", "file", compressedFilePath)
		// A prior attempt produced the compressed output but may have crashed before removing the
		// plain source. Drop it now so the next List/retention pass does not see a duplicate.
		if err := os.Remove(plainFilePath); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("error removing stale plain backup %s: %w", plainFilePath, err)
		}
		return compressedFileName, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("error stat-ing compressed backup %s: %w", compressedFilePath, err)
	}

	tmpFilePath := compressedFilePath + ".tmp"
	logger.Info("compressing file", "src", plainFilePath, "dst", compressedFilePath)

	// tmpFilePath must be closed before renaming. See: https://github.com/mariadb-operator/mariadb-operator/issues/1007
	if err := func() error {
		plainFile, err := os.Open(plainFilePath)
		if err != nil {
			return err
		}
		defer plainFile.Close()

		compressedFile, err := os.Create(tmpFilePath)
		if err != nil {
			return err
		}
		defer compressedFile.Close()

		// @PERF: Potential improvement here if we want this to be cancellable, can change to Background if we don't want to
		return compressor.Compress(context.TODO(), compressedFile, plainFile)
	}(); err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		if rmErr := os.Remove(tmpFilePath); rmErr != nil && !os.IsNotExist(rmErr) {
			errBundle = multierror.Append(errBundle, rmErr)
		}
		return "", errBundle
	}

	// Rename tmp -> final BEFORE removing the plain source so there is never a window where neither
	// file exists. If we crash between Rename and Remove, the idempotency check above handles it.
	if err := os.Rename(tmpFilePath, compressedFilePath); err != nil {
		return "", err
	}
	if err := os.Remove(plainFilePath); err != nil {
		return "", err
	}
	return compressedFileName, nil
}

func decompressFile(path, fileName string, logger logr.Logger, getUncompressedFilename GetBackupUncompressedFilenameFn,
	compressor Compressor) (string, error) {
	filePath := getFilePath(path, fileName)
	logger.Info("decompressing file", "file", filePath)

	compressedFile, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer compressedFile.Close()

	plainFileName, err := getUncompressedFilename(fileName)
	if err != nil {
		return "", err
	}
	plainFilePath := getFilePath(path, plainFileName)
	plainFile, err := os.Create(plainFilePath)
	if err != nil {
		return "", err
	}
	defer plainFile.Close()

	if err := compressor.Decompress(context.TODO(), plainFile, compressedFile); err != nil {
		return "", err
	}

	return plainFilePath, nil
}

func getFilePath(basePath, fileName string) string {
	if filepath.IsAbs(fileName) {
		return fileName
	}
	return filepath.Join(basePath, fileName)
}
