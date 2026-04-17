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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"k8s.io/utils/ptr"
)

type MinioOpts struct {
	CredsProviders      []credentials.Provider
	TLS                 bool
	CACertPath          string
	CACertBytes         []byte
	Region              string
	Prefix              string
	AllowNestedPrefixes bool
	SSECCustomerKey     string
}

func (o *MinioOpts) getCredentials() *credentials.Credentials {
	// Use a chained credentials provider to support multiple sources:
	// 1. Credentials providers passed as functional option
	// 2. Environment variables (set by custom resource)
	// 3. IAM role (for EC2 Meta Data, EKS service accounts when environment variables are not set)
	providers := o.CredsProviders
	providers = append(providers, []credentials.Provider{
		&credentials.EnvAWS{},
		&credentials.IAM{},
	}...)
	return credentials.NewChainCredentials(providers)
}

type MinioOpt func(m *MinioOpts)

func WithCredsProviders(provider ...credentials.Provider) MinioOpt {
	return func(m *MinioOpts) {
		m.CredsProviders = provider
	}
}

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

func WithCACertBytes(bytes []byte) MinioOpt {
	return func(m *MinioOpts) {
		m.CACertBytes = bytes
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

func WithAllowNestedPrefixes(allowNestedPrefixes bool) MinioOpt {
	return func(m *MinioOpts) {
		m.AllowNestedPrefixes = allowNestedPrefixes
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

func NewMinioClientFromS3Config(
	ctx context.Context,
	refResolver refresolver.RefResolver,
	s3 v1alpha1.S3,
	basePath,
	namespace string,
	mOpts ...MinioOpt,
) (*Client, error) {
	minioOpts := []MinioOpt{
		WithRegion(s3.Region),
		WithPrefix(s3.Prefix),
	}

	if s3.AccessKeyIdSecretKeyRef != nil && s3.SecretAccessKeySecretKeyRef != nil {
		accessKeyID, err := refResolver.SecretKeyRef(ctx, *s3.AccessKeyIdSecretKeyRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting S3 access key ID: %v", err)
		}
		secretAccessKey, err := refResolver.SecretKeyRef(ctx, *s3.SecretAccessKeySecretKeyRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting S3 access key ID: %v", err)
		}
		var sessionToken string
		if s3.SessionTokenSecretKeyRef != nil {
			sessionToken, err = refResolver.SecretKeyRef(ctx, *s3.SessionTokenSecretKeyRef, namespace)
			if err != nil {
				return nil, fmt.Errorf("error getting S3 session token: %v", err)
			}
		}
		minioOpts = append(minioOpts, WithCredsProviders(&credentials.Static{
			Value: credentials.Value{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				SessionToken:    sessionToken,
				SignerType:      credentials.SignatureDefault,
			},
		}))
	}

	tls := ptr.Deref(s3.TLS, v1alpha1.TLSConfig{})
	if tls.Enabled {
		minioOpts = append(minioOpts, WithTLS(true))
		caCertBytes, err := refResolver.SecretKeyRef(ctx, *s3.TLS.CASecretKeyRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting CA cert: %v", err)
		}
		minioOpts = append(minioOpts, WithCACertBytes([]byte(caCertBytes)))
	}

	if s3.SSEC != nil {
		ssecKey, err := refResolver.SecretKeyRef(ctx, s3.SSEC.CustomerKeySecretKeyRef, namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting SSEC key: %v", err)
		}
		minioOpts = append(minioOpts, WithSSECCustomerKey(ssecKey))
	}

	minioOpts = append(minioOpts, mOpts...)

	s3Client, err := NewMinioClient(
		basePath,
		s3.Bucket,
		s3.Endpoint,
		minioOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 client: %v", err)
	}
	return s3Client, nil
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

func (c *Client) ListObjectsWithOptions(ctx context.Context) ([]string, error) {
	var fileNames []string
	for o := range c.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix: c.GetPrefix(),
	}) {
		if o.Err != nil {
			return nil, o.Err
		}
		fileNames = append(fileNames, o.Key)
	}
	return fileNames, nil
}

func (c *Client) PutObjectWithOptions(ctx context.Context, fileName string, reader io.Reader, size int64) error {
	putOpts, err := c.putObjectOptions()
	if err != nil {
		return err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)

	_, err = c.PutObject(ctx, c.bucket, prefixedFilePath, reader, size, *putOpts)
	return err
}

func (c *Client) FPutObjectWithOptions(ctx context.Context, fileName string) error {
	putOpts, err := c.putObjectOptions()
	if err != nil {
		return err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)
	filePath := c.getFilePath(fileName)

	_, err = c.FPutObject(ctx, c.bucket, prefixedFilePath, filePath, *putOpts)
	return err
}

func (c *Client) GetObjectWithOptions(ctx context.Context, fileName string) (io.ReadCloser, error) {
	getOpts, err := c.getObjectOptions()
	if err != nil {
		return nil, err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)

	return c.GetObject(ctx, c.bucket, prefixedFilePath, *getOpts)
}

func (c *Client) FGetObjectWithOptions(ctx context.Context, fileName string) error {
	getOpts, err := c.getObjectOptions()
	if err != nil {
		return err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)
	filePath := c.getFilePath(fileName)

	return c.FGetObject(ctx, c.bucket, prefixedFilePath, filePath, *getOpts)
}

func (c *Client) RemoveWithOptions(ctx context.Context, fileName string) error {
	prefixedFilePath := c.PrefixedFileName(fileName)
	return c.RemoveObject(ctx, c.bucket, prefixedFilePath, minio.RemoveObjectOptions{})
}

func (c *Client) IsNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	if resp.StatusCode == http.StatusNotFound {
		return true
	}
	switch resp.Code {
	case "NoSuchKey", "NotFound":
		return true
	}
	return false
}

func (c *Client) Exists(ctx context.Context, fileName string) (bool, error) {
	statOpts, err := c.getObjectOptions()
	if err != nil {
		return false, err
	}

	prefixedFilePath := c.PrefixedFileName(fileName)

	_, err = c.StatObject(ctx, c.bucket, prefixedFilePath, *statOpts)
	if err != nil {
		if c.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) PrefixedFileName(fileName string) string {
	if c.AllowNestedPrefixes {
		return c.GetPrefix() + fileName
	}
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

func (c *Client) IsAuthenticated(ctx context.Context) bool {
	val, err := c.getCredentials().GetWithContext(nil)
	return err == nil && val != (credentials.Value{})
}

func (c *Client) GetCredentials() *credentials.Credentials {
	return c.getCredentials()
}

func (c *Client) putObjectOptions() (*minio.PutObjectOptions, error) {
	putOpts := minio.PutObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return nil, fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		putOpts.ServerSideEncryption = sse
	}
	return &putOpts, nil
}

func (c *Client) getObjectOptions() (*minio.GetObjectOptions, error) {
	getOpts := minio.GetObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return nil, fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		getOpts.ServerSideEncryption = sse
	}
	return &getOpts, nil
}

// StatObjectOptions are the same as normal get ones and do not provide extra functionality
// e.g.: type StatObjectOptions = GetObjectOptions
func (c *Client) StatObjectOptions() (*minio.GetObjectOptions, error) {
	return c.getObjectOptions()
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
	return &minio.Options{
		Creds:     opts.getCredentials(),
		Region:    opts.Region,
		Secure:    opts.TLS,
		Transport: transport,
	}, nil
}

// GetObjectStream returns a streaming reader for an S3 object along with its size.
func (c *Client) GetObjectStream(ctx context.Context, fileName string) (*minio.Object, int64, error) {
	getOpts, err := c.getObjectOptions()
	if err != nil {
		return nil, 0, err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)

	obj, err := c.GetObject(ctx, c.bucket, prefixedFilePath, *getOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting object: %v", err)
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("error getting object stat: %v", err)
	}
	return obj, stat.Size, nil
}

// ResumableReaderConfig configures the retry behavior of ResumableReader.
type ResumableReaderConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Logger         logr.Logger
}

// DefaultResumableReaderConfig returns reasonable defaults for resumable reads.
func DefaultResumableReaderConfig(logger logr.Logger) ResumableReaderConfig {
	return ResumableReaderConfig{
		MaxRetries:     10,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     60 * time.Second,
		Logger:         logger,
	}
}

// ResumableReader is a thread-safe io.ReadCloser that automatically resumes S3 reads
// on transient network errors using Range headers.
type ResumableReader struct {
	client        *Client
	ctx           context.Context
	bucket        string
	objectKey     string
	totalSize     int64
	position      int64
	currentReader io.ReadCloser
	config        ResumableReaderConfig
	mu            sync.Mutex
	closed        bool
	retryCount    int
	totalRetries  int
	lastError     error
	getOpts       minio.GetObjectOptions
}

// GetResumableObjectStream returns a ResumableReader that automatically retries and resumes
// reads from the given position on transient errors.
func (c *Client) GetResumableObjectStream(ctx context.Context, fileName string,
	config ResumableReaderConfig) (*ResumableReader, int64, error) {
	getOpts, err := c.getObjectOptions()
	if err != nil {
		return nil, 0, err
	}
	prefixedFilePath := c.PrefixedFileName(fileName)

	obj, err := c.GetObject(ctx, c.bucket, prefixedFilePath, *getOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting object: %v", err)
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("error getting object stat: %v", err)
	}

	return &ResumableReader{
		client:        c,
		ctx:           ctx,
		bucket:        c.bucket,
		objectKey:     prefixedFilePath,
		totalSize:     stat.Size,
		position:      0,
		currentReader: obj,
		config:        config,
		getOpts:       *getOpts,
	}, stat.Size, nil
}

func (r *ResumableReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return 0, errors.New("reader is closed")
	}
	if err := r.ctx.Err(); err != nil {
		r.mu.Unlock()
		return 0, err
	}
	reader := r.currentReader
	r.mu.Unlock()

	n, err := reader.Read(p)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.position += int64(n)

	if err == nil {
		r.retryCount = 0
		return n, nil
	}

	if err == io.EOF {
		return n, io.EOF
	}

	if r.retryCount >= r.config.MaxRetries {
		r.lastError = err
		return n, fmt.Errorf("max retries (%d) exceeded, last error: %w", r.config.MaxRetries, err)
	}

	r.retryCount++
	r.totalRetries++
	r.config.Logger.Info("Read error, attempting to resume",
		"error", err,
		"position", r.position,
		"totalSize", r.totalSize,
		"retry", r.retryCount,
		"progress", fmt.Sprintf("%.1f%%", float64(r.position)/float64(r.totalSize)*100),
	)

	if resumeErr := r.resumeFromPositionLocked(); resumeErr != nil {
		r.lastError = resumeErr
		return n, fmt.Errorf("error resuming from position %d: %w", r.position, resumeErr)
	}

	return n, nil
}

func (r *ResumableReader) resumeFromPositionLocked() error {
	if r.currentReader != nil {
		r.currentReader.Close()
		r.currentReader = nil
	}

	backoff := r.config.InitialBackoff * time.Duration(1<<(r.retryCount-1))
	if backoff > r.config.MaxBackoff {
		backoff = r.config.MaxBackoff
	}

	r.config.Logger.Info("Waiting before resume",
		"backoff", backoff,
		"position", r.position,
	)

	// Release lock during sleep so the reader can be closed concurrently.
	r.mu.Unlock()
	select {
	case <-time.After(backoff):
	case <-r.ctx.Done():
		r.mu.Lock()
		return r.ctx.Err()
	}
	r.mu.Lock()

	if r.closed {
		return errors.New("reader closed during backoff")
	}

	getOpts := r.getOpts
	if err := getOpts.SetRange(r.position, r.totalSize-1); err != nil {
		return fmt.Errorf("error setting range header: %v", err)
	}

	obj, err := r.client.GetObject(r.ctx, r.bucket, r.objectKey, getOpts)
	if err != nil {
		return fmt.Errorf("error getting object for resume: %v", err)
	}

	r.currentReader = obj
	r.config.Logger.Info("Successfully resumed read",
		"position", r.position,
		"totalSize", r.totalSize,
		"progress", fmt.Sprintf("%.1f%%", float64(r.position)/float64(r.totalSize)*100),
	)
	return nil
}

// Position returns the current read position (thread-safe).
func (r *ResumableReader) Position() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.position
}

// TotalRetries returns the total number of retries performed (thread-safe).
func (r *ResumableReader) TotalRetries() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalRetries
}

// Close closes the ResumableReader and releases resources.
func (r *ResumableReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.currentReader != nil {
		return r.currentReader.Close()
	}
	return nil
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

	if opts.CACertBytes != nil {
		if ok := transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(opts.CACertBytes); !ok {
			return nil, errors.New("unable to add CA cert to pool")
		}
	} else if opts.CACertPath != "" {
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
