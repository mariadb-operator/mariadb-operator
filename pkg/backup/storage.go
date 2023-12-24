package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type BackupStorage interface {
	List(ctx context.Context) ([]string, error)
	Push(ctx context.Context, fileName string) error
	Pull(ctx context.Context, fileName string) error
	Delete(ctx context.Context, fileName string) error
}

type FileSystemBackupStorage struct {
	basePath string
	logger   logr.Logger
}

func NewFileSystemBackupStorage(basePath string, logger logr.Logger) BackupStorage {
	return &FileSystemBackupStorage{
		basePath: basePath,
		logger:   logger,
	}
}

func (f *FileSystemBackupStorage) List(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, e := range entries {
		name := e.Name()
		f.logger.V(1).Info("processing backup file", "file", name)
		if IsValidBackupFile(name) {
			fileNames = append(fileNames, name)
		} else {
			f.logger.V(1).Info("ignoring file", "file", name)
		}
	}
	return fileNames, nil
}

func (f *FileSystemBackupStorage) Push(ctx context.Context, fileName string) error {
	return nil
}

func (f *FileSystemBackupStorage) Pull(ctx context.Context, fileName string) error {
	return nil
}

func (f *FileSystemBackupStorage) Delete(ctx context.Context, fileName string) error {
	return os.Remove(filepath.Join(f.basePath, fileName))
}

type S3BackupStorage struct {
	basePath string
	bucket   string
	client   *minio.Client
	logger   logr.Logger
}

func NewS3BackupStorage(basePath, bucket, endpointURL, accessKeyID, secretAccessKey string, logger logr.Logger) (BackupStorage, error) {
	client, err := minio.New(endpointURL, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating S3 client: %v", err)
	}
	return &S3BackupStorage{
		basePath: basePath,
		bucket:   bucket,
		client:   client,
		logger:   logger,
	}, nil
}

func (s *S3BackupStorage) List(ctx context.Context) ([]string, error) {
	var objs []string
	for o := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{}) {
		objs = append(objs, o.Key)
	}
	return objs, nil
}

func (s *S3BackupStorage) Push(ctx context.Context, fileName string) error {
	filePath := filepath.Join(s.basePath, fileName)
	_, err := s.client.FPutObject(ctx, s.bucket, fileName, filePath, minio.PutObjectOptions{})
	return err
}

func (s *S3BackupStorage) Pull(ctx context.Context, fileName string) error {
	filePath := filepath.Join(s.basePath, fileName)
	return s.client.FGetObject(ctx, s.bucket, fileName, filePath, minio.GetObjectOptions{})
}

func (s *S3BackupStorage) Delete(ctx context.Context, fileName string) error {
	return s.client.RemoveObject(ctx, s.bucket, fileName, minio.RemoveObjectOptions{})
}
