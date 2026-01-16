package compression

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dsnet/compress/bzip2"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/klauspost/pgzip"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
)

type Compressor interface {
	Compress(fileName string) error
	Decompress(fileName string) (string, error)
}

// StreamingCompressor extends Compressor with streaming decompression capabilities
// that allow direct piping of compressed data without intermediate files.
type StreamingCompressor interface {
	Compressor
	// DecompressStream reads compressed data from src and writes decompressed data to dst.
	// Returns the number of bytes written and any error encountered.
	DecompressStream(dst io.Writer, src io.Reader) (int64, error)
}

// Compile-time interface verification
var (
	_ StreamingCompressor = (*NopCompressor)(nil)
	_ StreamingCompressor = (*GzipBackupCompressor)(nil)
	_ StreamingCompressor = (*Bzip2BackupCompressor)(nil)
)

type GetUncompressedFilenameFn func(compressedFilename string) (string, error)

func NewCompressor(calg mariadbv1alpha1.CompressAlgorithm, basePath string,
	getUncompressedFilename GetUncompressedFilenameFn, logger logr.Logger) (Compressor, error) {
	switch calg {
	case mariadbv1alpha1.CompressNone:
		return NewNopCompressor(basePath, getUncompressedFilename, logger.WithName("nop-compressor")), nil
	case mariadbv1alpha1.CompressGzip:
		return NewGzipBackupCompressor(basePath, getUncompressedFilename, logger.WithName("gzip-compressor")), nil
	case mariadbv1alpha1.CompressBzip2:
		return NewBzip2BackupCompressor(basePath, getUncompressedFilename, logger.WithName("bzip2-compressor")), nil
	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %v", calg)
	}
}

type NopCompressor struct {
	basePath string
}

func NewNopCompressor(basePath string, getUncompressedFilename GetUncompressedFilenameFn, logger logr.Logger) Compressor {
	return &NopCompressor{
		basePath: basePath,
	}
}

func (c *NopCompressor) Compress(fileName string) error {
	return nil
}

func (c *NopCompressor) Decompress(fileName string) (string, error) {
	return getFilePath(c.basePath, fileName), nil
}

func (c *NopCompressor) DecompressStream(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

type GzipBackupCompressor struct {
	basePath                string
	getUncompressedFilename GetUncompressedFilenameFn
	logger                  logr.Logger
}

func NewGzipBackupCompressor(basePath string, getUncompressedFilename GetUncompressedFilenameFn, logger logr.Logger) Compressor {
	return &GzipBackupCompressor{
		basePath:                basePath,
		getUncompressedFilename: getUncompressedFilename,
		logger:                  logger,
	}
}

func (c *GzipBackupCompressor) Compress(fileName string) error {
	return compressFile(c.basePath, fileName, c.logger, func(dst io.Writer, src io.Reader) error {
		writer := gzip.NewWriter(dst)
		defer writer.Close()
		_, err := io.Copy(writer, src)
		return err
	})
}

func (c *GzipBackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, func(dst io.Writer, src io.Reader) error {
		reader, err := gzip.NewReader(src)
		if err != nil {
			return err
		}
		defer reader.Close()
		_, err = io.Copy(dst, reader)
		return err
	})
}

func (c *GzipBackupCompressor) DecompressStream(dst io.Writer, src io.Reader) (int64, error) {
	reader, err := pgzip.NewReader(src)
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	return io.Copy(dst, reader)
}

type Bzip2BackupCompressor struct {
	basePath                string
	getUncompressedFilename GetUncompressedFilenameFn
	logger                  logr.Logger
}

func NewBzip2BackupCompressor(basePath string, getUncompressedFilename GetUncompressedFilenameFn, logger logr.Logger) Compressor {
	return &Bzip2BackupCompressor{
		basePath:                basePath,
		getUncompressedFilename: getUncompressedFilename,
		logger:                  logger,
	}
}

func (c *Bzip2BackupCompressor) Compress(fileName string) error {
	return compressFile(c.basePath, fileName, c.logger, func(dst io.Writer, src io.Reader) error {
		writer, err := bzip2.NewWriter(dst,
			&bzip2.WriterConfig{Level: bzip2.DefaultCompression})
		if err != nil {
			return err
		}
		defer writer.Close()
		_, err = io.Copy(writer, src)
		return err
	})
}

func (c *Bzip2BackupCompressor) Decompress(fileName string) (string, error) {
	return decompressFile(c.basePath, fileName, c.logger, c.getUncompressedFilename, func(dst io.Writer, src io.Reader) error {
		reader, err := bzip2.NewReader(src,
			&bzip2.ReaderConfig{})
		if err != nil {
			return err
		}
		defer reader.Close()
		_, err = io.Copy(dst, reader)
		return err
	})
}

func (c *Bzip2BackupCompressor) DecompressStream(dst io.Writer, src io.Reader) (int64, error) {
	reader, err := bzip2.NewReader(src, &bzip2.ReaderConfig{})
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	return io.Copy(dst, reader)
}

func compressFile(path, fileName string, logger logr.Logger, compressFn func(dst io.Writer, src io.Reader) error) error {
	filePath := getFilePath(path, fileName)
	logger.Info("compressing file", "file", filePath)

	compressedFilePath := filePath + ".tmp"

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

		return compressFn(compressedFile, plainFile)
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

func decompressFile(path, fileName string, logger logr.Logger, getUncompressedFilename GetUncompressedFilenameFn,
	uncompressFn func(dst io.Writer, src io.Reader) error) (string, error) {
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

	if err := uncompressFn(plainFile, compressedFile); err != nil {
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
