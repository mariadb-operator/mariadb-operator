package backup

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/minio/minio-go/v7"
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

type S3BackupStorage struct {
	bucket    string
	processor BackupProcessor
	logger    logr.Logger
	client    *mariadbminio.Client
}

func NewS3BackupStorage(basePath, bucket, endpoint string, processor BackupProcessor, logger logr.Logger,
	mOpts ...mariadbminio.MinioOpt) (BackupStorage, error) {
	client, err := mariadbminio.NewMinioClient(basePath, bucket, endpoint, mOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating S3 client: %v", err)
	}

	return &S3BackupStorage{
		bucket:    bucket,
		client:    client,
		processor: processor,
		logger:    logger,
	}, nil
}

func (s *S3BackupStorage) List(ctx context.Context) ([]string, error) {
	var fileNames []string
	for o := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix: s.client.GetPrefix(),
	}) {
		if o.Err != nil {
			return nil, o.Err
		}
		fileName := o.Key
		if s.shouldProcessBackupFile(fileName, s.logger) {
			fileNames = append(fileNames, fileName)
		}
	}
	return fileNames, nil
}

func (s *S3BackupStorage) Push(ctx context.Context, fileName string) error {
	return s.client.FPutObjectWithOptions(ctx, fileName)
}

func (s *S3BackupStorage) Pull(ctx context.Context, fileName string) error {
	return s.client.FGetObjectWithOptions(ctx, fileName)
}

func (s *S3BackupStorage) Delete(ctx context.Context, fileName string) error {
	return s.client.RemoveWithOptions(ctx, fileName)
}

func (s *S3BackupStorage) shouldProcessBackupFile(fileName string, logger logr.Logger) bool {
	logger.V(1).Info("processing backup file", "file", fileName)
	if s.processor.IsValidBackupFile(s.client.UnprefixedFilename(fileName)) {
		return true
	}
	logger.V(1).Info("ignoring file", "file", fileName)
	return false
}
