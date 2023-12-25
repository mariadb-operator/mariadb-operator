package backup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
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

type S3BackupStorageOpts struct {
	TLS        bool
	CACertPath string
}

type S3BackupStorageOpt func(s *S3BackupStorageOpts)

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

func NewS3BackupStorage(basePath, bucket, endpointURL, accessKeyID, secretAccessKey string,
	logger logr.Logger, s3Opts ...S3BackupStorageOpt) (BackupStorage, error) {
	opts := S3BackupStorageOpts{}
	for _, setOpt := range s3Opts {
		setOpt(&opts)
	}

	minioOpts, err := getMinioOptions(accessKeyID, secretAccessKey, opts)
	if err != nil {
		return nil, fmt.Errorf("error creating S3 client options: %v", err)
	}

	client, err := minio.New(endpointURL, minioOpts)
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

func getMinioOptions(accessKeyID, secretAccessKey string, opts S3BackupStorageOpts) (*minio.Options, error) {
	minioOpts := &minio.Options{
		Creds: credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
	}
	if opts.TLS {
		minioOpts.Secure = true

		bytes, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading CA cert: %v", err)
		}
		rootCAs := x509.NewCertPool()
		cert, err := pki.ParseCert(bytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing CA cert: %v", err)
		}
		rootCAs.AddCert(cert)

		minioOpts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCAs,
			},
		}
	}
	return minioOpts, nil
}
