package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/pkg/minio"
	"github.com/minio/minio-go/v7"
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
		fileName := e.Name()
		if shouldProcessBackupFile(fileName, f.logger) {
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
	return os.Remove(filepath.Join(f.basePath, fileName))
}

type S3BackupStorageOpts struct {
	Region     string
	Prefix     string
	TLS        bool
	CACertPath string
}

type S3BackupStorageOpt func(s *S3BackupStorageOpts)

func WithRegion(region string) S3BackupStorageOpt {
	return func(s *S3BackupStorageOpts) {
		s.Region = region
	}
}

func WithPrefix(prefix string) S3BackupStorageOpt {
	return func(s *S3BackupStorageOpts) {
		s.Prefix = prefix
	}
}

func WithTLS(caCertPath string) S3BackupStorageOpt {
	return func(s *S3BackupStorageOpts) {
		s.TLS = true
		s.CACertPath = caCertPath
	}
}

type S3BackupStorage struct {
	S3BackupStorageOpts
	basePath string
	bucket   string
	logger   logr.Logger
	client   *minio.Client
}

func NewS3BackupStorage(basePath, bucket, endpoint string, logger logr.Logger, s3Opts ...S3BackupStorageOpt) (BackupStorage, error) {
	opts := S3BackupStorageOpts{}
	for _, setOpt := range s3Opts {
		setOpt(&opts)
	}

	clientOpts := []mariadbminio.MinioOpt{
		mariadbminio.WithRegion(opts.Region),
	}
	if opts.TLS {
		clientOpts = append(clientOpts, mariadbminio.WithTLS(opts.CACertPath))
	}
	client, err := mariadbminio.NewMinioClient(endpoint, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating S3 client: %v", err)
	}

	return &S3BackupStorage{
		S3BackupStorageOpts: opts,
		basePath:            basePath,
		bucket:              bucket,
		client:              client,
		logger:              logger,
	}, nil
}

func (s *S3BackupStorage) List(ctx context.Context) ([]string, error) {
	var fileNames []string
	for o := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix: s.Prefix,
	}) {
		fileName := o.Key
		if shouldProcessBackupFile(fileName, s.logger) {
			fileNames = append(fileNames, fileName)
		}
	}
	return fileNames, nil
}

func (s *S3BackupStorage) Push(ctx context.Context, fileName string) error {
	filePath := filepath.Join(s.basePath, fileName)
	_, err := s.client.FPutObject(ctx, s.bucket, s.prefixedFileName(fileName), filePath, minio.PutObjectOptions{})
	return err
}

func (s *S3BackupStorage) Pull(ctx context.Context, fileName string) error {
	filePath := filepath.Join(s.basePath, fileName)
	return s.client.FGetObject(ctx, s.bucket, s.prefixedFileName(fileName), filePath, minio.GetObjectOptions{})
}

func (s *S3BackupStorage) Delete(ctx context.Context, fileName string) error {
	return s.client.RemoveObject(ctx, s.bucket, s.prefixedFileName(fileName), minio.RemoveObjectOptions{})
}

func (s *S3BackupStorage) prefixedFileName(fileName string) string {
	if s.Prefix == "" {
		return fileName
	}
	prefix := s.Prefix
	if !strings.HasSuffix("/", prefix) {
		prefix += "/"
	}
	return prefix + fileName
}

func shouldProcessBackupFile(fileName string, logger logr.Logger) bool {
	logger.V(1).Info("processing backup file", "file", fileName)
	if IsValidBackupFile(fileName) {
		return true
	}
	logger.V(1).Info("ignoring file", "file", fileName)
	return false
}
