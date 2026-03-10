package compression

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
)

type BackupCompressOptions struct {
	compressedFileName string
}

type BackupCompressOpt func(*BackupCompressOptions)

func BackupWithCompressedFilename(compressedFileName string) BackupCompressOpt {
	return func(co *BackupCompressOptions) {
		co.compressedFileName = compressedFileName
	}
}

type BackupCompressor interface {
	Compress(fileName string, opts ...BackupCompressOpt) error
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

func (c *NopBackupCompressor) Compress(fileName string, opts ...BackupCompressOpt) error {
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

func (c *GzipBackupCompressor) Compress(fileName string, opts ...BackupCompressOpt) error {
	return compressFile(c.basePath, fileName, c.logger, c.compressor, opts...)
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

func (c *Bzip2BackupCompressor) Compress(fileName string, opts ...BackupCompressOpt) error {
	return compressFile(c.basePath, fileName, c.logger, c.compressor, opts...)
}

func (c *Bzip2BackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, c.compressor)
}

func compressFile(path, fileName string, logger logr.Logger, compressor Compressor, compressOpts ...BackupCompressOpt) error {
	opts := BackupCompressOptions{}
	for _, setOpt := range compressOpts {
		setOpt(&opts)
	}
	filePath := getFilePath(path, fileName)

	var compressedFilePath string
	if opts.compressedFileName != "" {
		if fileName == opts.compressedFileName {
			return errors.New("compressed file name must be different from plain file name")
		}
		compressedFilePath = getFilePath(path, opts.compressedFileName)
	} else {
		compressedFilePath = filePath + ".tmp"
	}
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

		return compressor.Compress(compressedFile, plainFile)
	}(); err != nil {
		var errBundle *multierror.Error
		errBundle = multierror.Append(errBundle, err)

		if err := os.Remove(compressedFilePath); err != nil && !os.IsNotExist(err) {
			errBundle = multierror.Append(errBundle, err)
		}
		return errBundle
	}

	if opts.compressedFileName == "" {
		if err := os.Remove(filePath); err != nil {
			return err
		}
		if err := os.Rename(compressedFilePath, filePath); err != nil {
			return err
		}
	}
	return nil
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

	if err := compressor.Decompress(plainFile, compressedFile); err != nil {
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
