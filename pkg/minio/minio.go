package minio

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"

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
	creds, err := getCredentials()
	if err != nil {
		return nil, fmt.Errorf("error getting credentials: %v", err)
	}
	transport, err := getTransport(&opts)
	if err != nil {
		return nil, fmt.Errorf("error getting transport: %v", err)
	}

	minioOpts := &minio.Options{
		Creds:     creds,
		Secure:    opts.TLS,
		Transport: transport,
	}
	return minioOpts, nil
}

func getCredentials() (*credentials.Credentials, error) {
	accessKeyID, secretAccessKey, err := readS3Credentials()
	if err != nil {
		return nil, err
	}
	return credentials.NewStaticV4(accessKeyID, secretAccessKey, ""), nil
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

func getTransport(opts *MinioOpts) (*http.Transport, error) {
	transport, err := minio.DefaultTransport(opts.TLS)
	if err != nil {
		return nil, err
	}
	if !opts.TLS {
		return transport, nil
	}

	if transport.TLSClientConfig.RootCAs == nil {
		pool, err := x509.SystemCertPool()
		if err != nil {
			transport.TLSClientConfig.RootCAs = x509.NewCertPool()
		} else {
			transport.TLSClientConfig.RootCAs = pool
		}
	}
	caBytes, err := os.ReadFile(opts.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA cert: %v", err)
	}
	if ok := transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(caBytes); !ok {
		return nil, fmt.Errorf("error parsing CA Certifiate : %s", err)
	}

	return transport, nil
}
