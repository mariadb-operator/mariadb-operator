package backup

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/azure"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
)

type BackupStorage interface {
	List(ctx context.Context) ([]string, error)
	Push(ctx context.Context, fileName string) error
	Pull(ctx context.Context, fileName string) error
	Delete(ctx context.Context, fileName string) error
	shouldProcessBackupFile(fileName string, logger logr.Logger) bool
}

type FileSystemBackupStorage struct {
	basePath  string
	processor BackupProcessor
	logger    logr.Logger
}

func NewFileSystemBackupStorage(basePath string, processor BackupProcessor, logger logr.Logger) BackupStorage {
	return &FileSystemBackupStorage{
		basePath:  basePath,
		processor: processor,
		logger:    logger,
	}
}

func (f *FileSystemBackupStorage) List(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, e := range entries {
		fileName := e.Name()
		if f.shouldProcessBackupFile(fileName, f.logger) {
			fileNames = append(fileNames, fileName)
		}
	}
	return fileNames, nil
}

func (f *FileSystemBackupStorage) Push(ctx context.Context, fileName string) error {
	return nil // noop
}

func (f *FileSystemBackupStorage) Pull(ctx context.Context, fileName string) error {
	return nil // noop
}

func (f *FileSystemBackupStorage) Delete(ctx context.Context, fileName string) error {
	return os.Remove(GetFilePath(f.basePath, fileName))
}

func (f *FileSystemBackupStorage) shouldProcessBackupFile(fileName string, logger logr.Logger) bool {
	logger.V(1).Info("processing backup file", "file", fileName)
	if f.processor.IsValidBackupFile(fileName) {
		return true
	}
	logger.V(1).Info("ignoring file", "file", fileName)
	return false
}

// PullStream returns a streaming reader for a local file along with its size.
func (f *FileSystemBackupStorage) PullStream(_ context.Context, fileName string) (io.ReadCloser, int64, error) {
	filePath := GetFilePath(f.basePath, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("error opening file: %v", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("error getting file info: %v", err)
	}
	return file, info.Size(), nil
}

// PullStreamResumable returns a streaming reader. For local files, this is equivalent to PullStream.
func (f *FileSystemBackupStorage) PullStreamResumable(ctx context.Context, fileName string,
	_ mariadbminio.ResumableReaderConfig) (io.ReadCloser, int64, error) {
	return f.PullStream(ctx, fileName)
}

type BlobBackupStorage struct {
	processor BackupProcessor
	client    interfaces.BlobStorage
}

func NewBlobBackupStorageWithS3(basePath, bucket, endpoint string, processor BackupProcessor,
	mOpts ...mariadbminio.MinioOpt) (BackupStorage, error) {
	client, err := mariadbminio.NewMinioClient(basePath, bucket, endpoint, mOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating S3 client: %v", err)
	}

	return &BlobBackupStorage{
		client:    client,
		processor: processor,
	}, nil
}

func NewBlobBackupStorageWithABS(basePath, containerName, serviceURL string, processor BackupProcessor,
	absOpts ...azure.AzBlobOpt) (BackupStorage, error) {
	client, err := azure.NewAzBlobClient(basePath, containerName, serviceURL, absOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating Azure Blob Client: %v", err)
	}

	return &BlobBackupStorage{
		client:    client,
		processor: processor,
	}, nil
}

func (s *BlobBackupStorage) Delete(ctx context.Context, fileName string) error {
	return s.client.RemoveWithOptions(ctx, fileName)
}

func (s *BlobBackupStorage) List(ctx context.Context) ([]string, error) {
	return s.client.ListObjectsWithOptions(ctx)
}

func (s *BlobBackupStorage) Push(ctx context.Context, fileName string) error {
	return s.client.FPutObjectWithOptions(ctx, fileName)
}

func (s *BlobBackupStorage) Pull(ctx context.Context, fileName string) error {
	return s.client.FGetObjectWithOptions(ctx, fileName)
}

func (s *BlobBackupStorage) shouldProcessBackupFile(fileName string, logger logr.Logger) bool {
	logger.V(1).Info("processing backup file", "file", fileName)
	if s.processor.IsValidBackupFile(s.client.UnprefixedFilename(fileName)) {
		return true
	}
	logger.V(1).Info("ignoring file", "file", fileName)
	return false
}

// PullStream returns a streaming reader for a blob storage object along with its size.
// For S3 clients, it uses StatObject for size. For other providers, size may be -1 (unknown).
func (s *BlobBackupStorage) PullStream(ctx context.Context, fileName string) (io.ReadCloser, int64, error) {
	if minioClient, ok := s.client.(*mariadbminio.Client); ok {
		return minioClient.GetObjectStream(ctx, fileName)
	}
	reader, err := s.client.GetObjectWithOptions(ctx, fileName)
	if err != nil {
		return nil, 0, err
	}
	return reader, -1, nil
}

// PullStreamResumable returns a resumable streaming reader for a blob storage object.
// For S3, it uses the ResumableReader with automatic retry. For other providers, it falls back to PullStream.
func (s *BlobBackupStorage) PullStreamResumable(ctx context.Context, fileName string,
	config mariadbminio.ResumableReaderConfig) (io.ReadCloser, int64, error) {
	if minioClient, ok := s.client.(*mariadbminio.Client); ok {
		return minioClient.GetResumableObjectStream(ctx, fileName, config)
	}
	return s.PullStream(ctx, fileName)
}
