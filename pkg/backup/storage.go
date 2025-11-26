package backup

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/encrypt"
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

type S3BackupStorageOpts struct {
	TLS             bool
	CACertPath      string
	Region          string
	Prefix          string
	SSECCustomerKey string
}

type S3BackupStorageOpt func(s *S3BackupStorageOpts)

func WithTLS(tls bool) S3BackupStorageOpt {
	return func(s *S3BackupStorageOpts) {
		s.TLS = tls
	}
}

func WithCACertPath(caCertPath string) S3BackupStorageOpt {
	return func(s *S3BackupStorageOpts) {
		s.CACertPath = caCertPath
	}
}

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

func WithSSECCustomerKey(ssecCustomerKey string) S3BackupStorageOpt {
	return func(s *S3BackupStorageOpts) {
		s.SSECCustomerKey = ssecCustomerKey
	}
}

type S3BackupStorage struct {
	S3BackupStorageOpts
	basePath  string
	bucket    string
	processor BackupProcessor
	logger    logr.Logger
	client    *minio.Client
}

func NewS3BackupStorage(basePath, bucket, endpoint string, processor BackupProcessor, logger logr.Logger,
	s3Opts ...S3BackupStorageOpt) (BackupStorage, error) {
	opts := S3BackupStorageOpts{}
	for _, setOpt := range s3Opts {
		setOpt(&opts)
	}

	clientOpts := []mariadbminio.MinioOpt{
		mariadbminio.WithTLS(opts.TLS),
		mariadbminio.WithCACertPath(opts.CACertPath),
		mariadbminio.WithRegion(opts.Region),
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
		processor:           processor,
		logger:              logger,
	}, nil
}

func (s *S3BackupStorage) List(ctx context.Context) ([]string, error) {
	var fileNames []string
	for o := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix: s.getPrefix(),
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
	s3FilePath := s.prefixedFileName(fileName)
	filePath := GetFilePath(s.basePath, fileName)
	putOpts := minio.PutObjectOptions{}
	if sse, err := s.getSSEC(); err != nil {
		return fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		putOpts.ServerSideEncryption = sse
	}
	_, err := s.client.FPutObject(ctx, s.bucket, s3FilePath, filePath, putOpts)
	return err
}

func (s *S3BackupStorage) Pull(ctx context.Context, fileName string) error {
	s3FilePath := s.prefixedFileName(fileName)
	filePath := GetFilePath(s.basePath, fileName)
	getOpts := minio.GetObjectOptions{}
	if sse, err := s.getSSEC(); err != nil {
		return fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		getOpts.ServerSideEncryption = sse
	}
	return s.client.FGetObject(ctx, s.bucket, s3FilePath, filePath, getOpts)
}

func (s *S3BackupStorage) Delete(ctx context.Context, fileName string) error {
	s3FilePath := s.prefixedFileName(fileName)
	return s.client.RemoveObject(ctx, s.bucket, s3FilePath, minio.RemoveObjectOptions{})
}

func (s *S3BackupStorage) shouldProcessBackupFile(fileName string, logger logr.Logger) bool {
	logger.V(1).Info("processing backup file", "file", fileName)
	if s.processor.IsValidBackupFile(s.unprefixedFilename(fileName)) {
		return true
	}
	logger.V(1).Info("ignoring file", "file", fileName)
	return false
}

func (s *S3BackupStorage) prefixedFileName(fileName string) string {
	return s.getPrefix() + filepath.Base(fileName)
}

func (s *S3BackupStorage) unprefixedFilename(fileName string) string {
	return strings.TrimPrefix(filepath.Base(fileName), s.getPrefix())
}

func (s *S3BackupStorage) getPrefix() string {
	if s.Prefix == "" || s.Prefix == "/" {
		return "" // object store doesn't use slash for root path
	}
	if !strings.HasSuffix(s.Prefix, "/") {
		return s.Prefix + "/" // ending slash is required for avoiding matching like "foo/" and "foobar/" with prefix "foo"
	}
	return s.Prefix
}

// getSSEC returns the SSE-C encryption object if SSECCustomerKey is configured.
// The key is expected to be base64 encoded and must be 32 bytes (256 bits) when decoded.
func (s *S3BackupStorage) getSSEC() (encrypt.ServerSide, error) {
	if s.SSECCustomerKey == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(s.SSECCustomerKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding SSE-C key from base64: %v", err)
	}
	sse, err := encrypt.NewSSEC(key)
	if err != nil {
		return nil, fmt.Errorf("error creating SSE-C encryption: %v", err)
	}
	return sse, nil
}
