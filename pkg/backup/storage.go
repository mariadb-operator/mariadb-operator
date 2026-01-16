package backup

import (
	"context"
	"fmt"
	"io"
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

// StreamingBackupStorage extends BackupStorage with streaming capabilities
// that allow direct piping of backup data without intermediate disk storage.
type StreamingBackupStorage interface {
	BackupStorage
	// PullStream returns a reader for the backup file content and its size.
	// The caller is responsible for closing the returned ReadCloser.
	PullStream(ctx context.Context, fileName string) (io.ReadCloser, int64, error)
	// PullStreamResumable returns a resumable reader that can automatically reconnect
	// using HTTP Range requests if the connection drops during download.
	// The caller is responsible for closing the returned ReadCloser.
	PullStreamResumable(ctx context.Context, fileName string, config mariadbminio.ResumableReaderConfig) (io.ReadCloser, int64, error)
}

// Compile-time interface verification
var (
	_ StreamingBackupStorage = (*FileSystemBackupStorage)(nil)
	_ StreamingBackupStorage = (*S3BackupStorage)(nil)
)

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

func (f *FileSystemBackupStorage) PullStream(ctx context.Context, fileName string) (io.ReadCloser, int64, error) {
	filePath, err := GetFilePathSafe(f.basePath, fileName)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid filename: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}

	return file, info.Size(), nil
}

// PullStreamResumable for filesystem just returns the regular stream since local files don't have network issues.
func (f *FileSystemBackupStorage) PullStreamResumable(ctx context.Context, fileName string, config mariadbminio.ResumableReaderConfig) (io.ReadCloser, int64, error) {
	return f.PullStream(ctx, fileName)
}

func (f *FileSystemBackupStorage) Delete(ctx context.Context, fileName string) error {
	filePath, err := GetFilePathSafe(f.basePath, fileName)
	if err != nil {
		return fmt.Errorf("invalid filename: %w", err)
	}
	return os.Remove(filePath)
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

func (s *S3BackupStorage) PullStream(ctx context.Context, fileName string) (io.ReadCloser, int64, error) {
	return s.client.GetObjectStream(ctx, fileName)
}

// PullStreamResumable returns a resumable reader that automatically reconnects using HTTP Range requests
// if the connection drops during download.
func (s *S3BackupStorage) PullStreamResumable(ctx context.Context, fileName string, config mariadbminio.ResumableReaderConfig) (io.ReadCloser, int64, error) {
	return s.client.GetResumableObjectStream(ctx, fileName, config)
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
