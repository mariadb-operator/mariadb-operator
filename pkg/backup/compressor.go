package backup

import (
	"compress/gzip"
	"io"
	"os"

	"github.com/dsnet/compress/bzip2"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
)

type BackupCompressor interface {
	Compress(fileName string) error
	Decompress(fileName string) (string, error)
}

type NopCompressor struct {
	basePath string
}

func NewNopCompressor(basePath string, processor BackupProcessor, logger logr.Logger) BackupCompressor {
	return &NopCompressor{
		basePath: basePath,
	}
}

func (c *NopCompressor) Compress(fileName string) error {
	return nil
}

func (c *NopCompressor) Decompress(fileName string) (string, error) {
	return GetFilePath(c.basePath, fileName), nil
}

type GzipBackupCompressor struct {
	basePath  string
	processor BackupProcessor
	logger    logr.Logger
}

func NewGzipBackupCompressor(basePath string, processor BackupProcessor, logger logr.Logger) BackupCompressor {
	return &GzipBackupCompressor{
		basePath:  basePath,
		processor: processor,
		logger:    logger,
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
	return decompressFile(c.basePath, fileName, c.processor, c.logger, func(dst io.Writer, src io.Reader) error {
		reader, err := gzip.NewReader(src)
		if err != nil {
			return err
		}
		defer reader.Close()
		_, err = io.Copy(dst, reader)
		return err
	})
}

type Bzip2BackupCompressor struct {
	basePath  string
	processor BackupProcessor
	logger    logr.Logger
}

func NewBzip2BackupCompressor(basePath string, processor BackupProcessor, logger logr.Logger) BackupCompressor {
	return &Bzip2BackupCompressor{
		basePath:  basePath,
		processor: processor,
		logger:    logger,
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
	return decompressFile(c.basePath, fileName, c.processor, c.logger, func(dst io.Writer, src io.Reader) error {
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

func compressFile(path, fileName string, logger logr.Logger, compressFn func(dst io.Writer, src io.Reader) error) error {
	filePath := GetFilePath(path, fileName)
	logger.Info("compressing backup", "file", filePath)

	compressedFilePath := filePath + ".tmp"

	// compressedFilePath must be closed before renaming. See: https://github.com/mariadb-operator/mariadb-operator/v25/issues/1007
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

func decompressFile(path, fileName string, processor BackupProcessor, logger logr.Logger,
	uncompressFn func(dst io.Writer, src io.Reader) error) (string, error) {
	filePath := GetFilePath(path, fileName)
	logger.Info("decompressing file", "file", filePath)

	compressedFile, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer compressedFile.Close()

	plainFileName, err := processor.GetUncompressedBackupFile(fileName)
	if err != nil {
		return "", err
	}
	plainFilePath := GetFilePath(path, plainFileName)
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
