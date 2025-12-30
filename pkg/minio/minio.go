package minio

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
)

type MinioOpts struct {
	TLS             bool
	CACertPath      string
	Region          string
	Prefix          string
	SSECCustomerKey string
}

type MinioOpt func(m *MinioOpts)

func WithTLS(tls bool) MinioOpt {
	return func(m *MinioOpts) {
		m.TLS = tls
	}
}

func WithCACertPath(caCertPath string) MinioOpt {
	return func(m *MinioOpts) {
		m.CACertPath = caCertPath
	}
}

func WithRegion(region string) MinioOpt {
	return func(m *MinioOpts) {
		m.Region = region
	}
}

func WithPrefix(prefix string) MinioOpt {
	return func(m *MinioOpts) {
		m.Prefix = prefix
	}
}

func WithSSECCustomerKey(ssecCustomerKey string) MinioOpt {
	return func(m *MinioOpts) {
		m.SSECCustomerKey = ssecCustomerKey
	}
}

type Client struct {
	*minio.Client
	MinioOpts
	basePath string
	bucket   string
}

func NewMinioClient(basePath, bucket, endpoint string, mOpts ...MinioOpt) (*Client, error) {
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
	return &Client{
		Client:    client,
		MinioOpts: opts,
		basePath:  basePath,
		bucket:    bucket,
	}, nil
}

func (c *Client) PutObjectWithOptions(ctx context.Context, fileName string, reader io.Reader, size int64) error {
	putOpts, err := c.getPutObjectOptions()
	if err != nil {
		return err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)

	_, err = c.PutObject(ctx, c.bucket, prefixedFilePath, reader, size, *putOpts)
	return err
}

func (c *Client) FPutObjectWithOptions(ctx context.Context, fileName string) error {
	putOpts, err := c.getPutObjectOptions()
	if err != nil {
		return err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)
	filePath := c.getFilePath(fileName)

	_, err = c.FPutObject(ctx, c.bucket, prefixedFilePath, filePath, *putOpts)
	return err
}

func (c *Client) FGetObjectWithOptions(ctx context.Context, fileName string) error {
	prefixedFilePath := c.PrefixedFileName(fileName)
	filePath := c.getFilePath(fileName)
	getOpts := minio.GetObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		getOpts.ServerSideEncryption = sse
	}

	return c.FGetObject(ctx, c.bucket, prefixedFilePath, filePath, getOpts)
}

func (c *Client) RemoveWithOptions(ctx context.Context, fileName string) error {
	prefixedFilePath := c.PrefixedFileName(fileName)
	return c.RemoveObject(ctx, c.bucket, prefixedFilePath, minio.RemoveObjectOptions{})
}

func (c *Client) PrefixedFileName(fileName string) string {
	return c.GetPrefix() + filepath.Base(fileName)
}

func (c *Client) UnprefixedFilename(fileName string) string {
	return strings.TrimPrefix(filepath.Base(fileName), c.GetPrefix())
}

func (c *Client) GetPrefix() string {
	if c.Prefix == "" || c.Prefix == "/" {
		return "" // object store doesn't use slash for root path
	}
	if !strings.HasSuffix(c.Prefix, "/") {
		return c.Prefix + "/" // ending slash is required for avoiding matching like "foo/" and "foobar/" with prefix "foo"
	}
	return c.Prefix
}

func (c *Client) getPutObjectOptions() (*minio.PutObjectOptions, error) {
	putOpts := minio.PutObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return nil, fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		putOpts.ServerSideEncryption = sse
	}
	return &putOpts, nil
}

func (c *Client) getFilePath(fileName string) string {
	if filepath.IsAbs(fileName) {
		return fileName
	}
	return filepath.Join(c.basePath, fileName)
}

// getSSEC returns the SSE-C encryption object if SSECCustomerKey is configured.
// The key is expected to be base64 encoded and must be 32 bytes (256 bits) when decoded.
func (c *Client) getSSEC() (encrypt.ServerSide, error) {
	if c.SSECCustomerKey == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(c.SSECCustomerKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding SSE-C key from base64: %v", err)
	}
	sse, err := encrypt.NewSSEC(key)
	if err != nil {
		return nil, fmt.Errorf("error creating SSE-C encryption: %v", err)
	}
	return sse, nil
}

func getMinioOptions(opts MinioOpts) (*minio.Options, error) {
	transport, err := getTransport(&opts)
	if err != nil {
		return nil, fmt.Errorf("error getting transport: %v", err)
	}

	// Use a chained credentials provider to support multiple sources:
	// 1. Environment variables (set by custom resource)
	// 2. IAM role (for EC2 Meta Data, EKS service accounts when environment variables are not set)
	chainedCreds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvAWS{},
			&credentials.IAM{},
		},
	)

	minioOpts := &minio.Options{
		Creds:     chainedCreds,
		Region:    opts.Region,
		Secure:    opts.TLS,
		Transport: transport,
	}
	return minioOpts, nil
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

	if opts.CACertPath != "" {
		caBytes, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading CA cert: %v", err)
		}
		if ok := transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(caBytes); !ok {
			return nil, errors.New("unable to add CA cert to pool")
		}
	}

	return transport, nil
}
