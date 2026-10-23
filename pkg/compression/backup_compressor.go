package compression

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

type BackupCompressor interface {
	Compress(fileName string) error
	Decompress(fileName string) (string, error)
}

type GetBackupUncompressedFilenameFn func(compressedFilename string) (string, error)

// Leading bytes of each compressed format. Backup inputs (xbstream "XBSTCK01", SQL dumps) never start with these.
var (
	gzipMagic  = []byte{0x1f, 0x8b}             // RFC 1952 §2.3.1 (ID1, ID2)
	bzip2Magic = []byte("BZh")                  // no RFC; see github.com/dsnet/compress/blob/master/doc/bzip2-format.pdf
	zstdMagic  = []byte{0x28, 0xb5, 0x2f, 0xfd} // RFC 8878 §3.1.1 (0xFD2FB528, little-endian)
)

func NewBackupCompressor(calg mariadbv1alpha1.CompressAlgorithm, threads int, basePath string,
	getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) (BackupCompressor, error) {
	switch calg {
	case mariadbv1alpha1.CompressNone:
		return NewNopBackupCompressor(basePath, getUncompressedFilename, logger.WithName("nop-compressor")), nil
	case mariadbv1alpha1.CompressGzip:
		return NewGzipBackupCompressor(basePath, getUncompressedFilename, logger.WithName("gzip-compressor")), nil
	case mariadbv1alpha1.CompressBzip2:
		return NewBzip2BackupCompressor(basePath, getUncompressedFilename, logger.WithName("bzip2-compressor")), nil
	case mariadbv1alpha1.CompressZstd:
		return NewZstdBackupCompressor(threads, basePath, getUncompressedFilename, logger.WithName("zstd-compressor")), nil
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

func (c *NopBackupCompressor) Compress(fileName string) error {
	return nil
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

func (c *GzipBackupCompressor) Compress(fileName string) error {
	return compressFile(c.basePath, fileName, gzipMagic, c.logger, c.compressor)
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

func (c *Bzip2BackupCompressor) Compress(fileName string) error {
	return compressFile(c.basePath, fileName, bzip2Magic, c.logger, c.compressor)
}

func (c *Bzip2BackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, c.compressor)
}

type ZstdBackupCompressor struct {
	compressor              *ZstdCompressor
	basePath                string
	getUncompressedFilename GetBackupUncompressedFilenameFn
	logger                  logr.Logger
}

func NewZstdBackupCompressor(threads int, basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn,
	logger logr.Logger) BackupCompressor {
	return &ZstdBackupCompressor{
		compressor:              &ZstdCompressor{concurrency: threads},
		basePath:                basePath,
		getUncompressedFilename: getUncompressedFilename,
		logger:                  logger,
	}
}

func (c *ZstdBackupCompressor) Compress(fileName string) error {
	return compressFile(c.basePath, fileName, zstdMagic, c.logger, c.compressor)
}

func (c *ZstdBackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, c.compressor)
}

// compressFile compresses in place. It is idempotent: the backup container may be restarted after compressing
// but before a successful push, and re-compressing would produce an unrestorable multi-layer archive.
func compressFile(path, fileName string, magic []byte, logger logr.Logger, compressor Compressor) error {
	filePath := getFilePath(path, fileName)
	compressed, err := hasPrefix(filePath, magic)
	if err != nil {
		return err
	}
	if compressed {
		logger.Info("file already compressed, skipping compression", "file", filePath)
		return nil
	}
	compressedFilePath := filePath + ".tmp"
	logger.Info("compressing file", "file", filePath)

	// compressedFilePath must be closed before renaming. See: https://github.com/mariadb-operator/mariadb-operator/issues/1007
	if err := func() error {
		plainFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer plainFile.Close()

		compressedFile, err := os.Create(compressedFilePath)
		if err != nil {
			return err
		}
		defer compressedFile.Close()

		// @PERF: Potential improvement here if we want this to be cancellable, can change to Background if we don't want to
		return compressor.Compress(context.TODO(), compressedFile, plainFile)
	}(); err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		if err := os.Remove(compressedFilePath); err != nil && !os.IsNotExist(err) {
			errBundle = multierror.Append(errBundle, err)
		}
		return errBundle
	}

	if err := os.Remove(filePath); err != nil {
		return err
	}
	if err := os.Rename(compressedFilePath, filePath); err != nil {
		return err
	}
	return nil
}

func hasPrefix(filePath string, prefix []byte) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, len(prefix))
	if _, err := io.ReadFull(f, buf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(buf, prefix), nil
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
