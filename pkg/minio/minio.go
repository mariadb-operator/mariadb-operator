package minio

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioOpts struct {
	TLS        bool
	CACertPath string
}

type MinioOpt func(m *MinioOpts)

func WithTLS(caCertPath string) MinioOpt {
	return func(s *MinioOpts) {
		s.TLS = true
		s.CACertPath = caCertPath
	}
}

func NewMinioClient(endpoint string, mOpts ...MinioOpt) (*minio.Client, error) {
	opts := MinioOpts{}
	for _, setOpt := range mOpts {
		setOpt(&opts)
	}

	minioOpts, err := getMinioOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("error creating Minio client options: %v", err)
	}
	client, err := minio.New(endpoint, minioOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating Minio client: %v", err)
	}
	return client, nil
}

func getMinioOptions(opts MinioOpts) (*minio.Options, error) {
	accessKeyID, secretAccessKey, err := readS3Credentials()
	if err != nil {
		return nil, err
	}
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

func readS3Credentials() (accessKeyID string, secretAccessKey string, err error) {
	accessKeyID = os.Getenv("S3_ACCESS_KEY_ID")
	if accessKeyID == "" {
		return "", "", errors.New("S3_ACCESS_KEY_ID must be set in order to authenticate with S3")
	}
	secretAccessKey = os.Getenv("S3_SECRET_ACCESS_KEY")
	if secretAccessKey == "" {
		return "", "", errors.New("S3_SECRET_ACCESS_KEY must be set in order to authenticate with S3")
	}
	return accessKeyID, secretAccessKey, nil
}
