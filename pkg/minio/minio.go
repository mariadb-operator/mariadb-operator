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

func (c *Client) FPutObjectWithOptions(ctx context.Context, fileName string) error {
	prefixedFilePath := c.PrefixedFileName(fileName)
	filePath := c.getFilePath(fileName)
	putOpts := minio.PutObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		putOpts.ServerSideEncryption = sse
	}

	_, err := c.FPutObject(ctx, c.bucket, prefixedFilePath, filePath, putOpts)
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

// GetObjectStream returns a streaming reader for an object in the bucket.
// The caller is responsible for closing the returned Object.
func (c *Client) GetObjectStream(ctx context.Context, fileName string) (*minio.Object, int64, error) {
	prefixedFilePath := c.PrefixedFileName(fileName)
	getOpts := minio.GetObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return nil, 0, fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		getOpts.ServerSideEncryption = sse
	}

	obj, err := c.GetObject(ctx, c.bucket, prefixedFilePath, getOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting object %s/%s: %v", c.bucket, prefixedFilePath, err)
	}
	if obj == nil {
		return nil, 0, fmt.Errorf("nil object returned for %s/%s", c.bucket, prefixedFilePath)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("error getting object stats for %s/%s: %v", c.bucket, prefixedFilePath, err)
	}

	return obj, info.Size, nil
}

// ResumableReaderConfig configures the resumable reader behavior.
type ResumableReaderConfig struct {
	MaxRetries     int           // Maximum number of retry attempts (default: 5)
	InitialBackoff time.Duration // Initial backoff duration (default: 1s)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 30s)
	Logger         logr.Logger   // Logger for retry information
}

// DefaultResumableReaderConfig returns a default configuration for resumable reader.
func DefaultResumableReaderConfig() ResumableReaderConfig {
	return ResumableReaderConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Logger:         logr.Discard(), // Safe no-op logger by default
	}
}

// ResumableReader wraps an S3 object reader with automatic resume capability.
// If a read fails, it automatically reconnects using HTTP Range requests to resume
// from where it left off.
type ResumableReader struct {
	client         *Client
	ctx            context.Context
	bucket         string
	objectKey      string
	totalSize      int64
	position       int64
	currentReader  io.ReadCloser
	config         ResumableReaderConfig
	mu             sync.Mutex
	closed         bool
	retryCount     int
	totalRetries   int
	lastError      error
	getOpts        minio.GetObjectOptions
}

// GetResumableObjectStream returns a resumable streaming reader for an object.
// If the connection drops during reading, it will automatically resume using Range requests.
func (c *Client) GetResumableObjectStream(ctx context.Context, fileName string, config ResumableReaderConfig) (*ResumableReader, int64, error) {
	prefixedFilePath := c.PrefixedFileName(fileName)
	getOpts := minio.GetObjectOptions{}
	if sse, err := c.getSSEC(); err != nil {
		return nil, 0, fmt.Errorf("error creating SSE-C encryption: %v", err)
	} else if sse != nil {
		getOpts.ServerSideEncryption = sse
	}

	// Get initial object to determine size
	obj, err := c.GetObject(ctx, c.bucket, prefixedFilePath, getOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting object %s/%s: %v", c.bucket, prefixedFilePath, err)
	}
	if obj == nil {
		return nil, 0, fmt.Errorf("nil object returned for %s/%s", c.bucket, prefixedFilePath)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("error getting object stats for %s/%s: %v", c.bucket, prefixedFilePath, err)
	}

	reader := &ResumableReader{
		client:        c,
		ctx:           ctx,
		bucket:        c.bucket,
		objectKey:     prefixedFilePath,
		totalSize:     info.Size,
		position:      0,
		currentReader: obj,
		config:        config,
		getOpts:       getOpts,
	}

	return reader, info.Size, nil
}

// Read implements io.Reader with automatic resume on failure.
// Note: This reader is designed for single-goroutine use. Concurrent Read/Close
// from different goroutines may block until the current read completes.
func (r *ResumableReader) Read(p []byte) (int, error) {
	for {
		// Acquire lock to check state and get current reader
		r.mu.Lock()

		// Check context cancellation first
		if r.ctx.Err() != nil {
			r.mu.Unlock()
			return 0, r.ctx.Err()
		}

		if r.closed {
			r.mu.Unlock()
			return 0, errors.New("reader is closed")
		}

		// Check if we've read everything
		if r.position >= r.totalSize {
			r.mu.Unlock()
			return 0, io.EOF
		}

		// Get current reader reference and release lock before blocking I/O
		currentReader := r.currentReader
		r.mu.Unlock()

		// Perform read without holding the lock
		n, err := currentReader.Read(p)

		// Re-acquire lock to update state
		r.mu.Lock()

		// Check if closed while we were reading (race with Close())
		if r.closed {
			r.mu.Unlock()
			// Return any data we got, but signal that reader is now closed
			if n > 0 {
				return n, errors.New("reader closed during read")
			}
			return 0, errors.New("reader closed during read")
		}

		if n > 0 {
			r.position += int64(n)
			r.retryCount = 0 // Reset retry count on successful read
		}

		// Handle read errors (not EOF)
		if err != nil && err != io.EOF {
			if r.retryCount < r.config.MaxRetries {
				r.lastError = err
				resumeErr := r.resumeFromPositionLocked()
				if resumeErr != nil {
					r.mu.Unlock()
					return n, fmt.Errorf("read error and resume failed: original=%w, resume=%w", err, resumeErr)
				}
				// Resume succeeded
				if n > 0 {
					r.mu.Unlock()
					return n, nil // Return data we got, caller will call Read again
				}
				// No data read, loop to try reading from new connection
				r.mu.Unlock()
				continue
			}
			r.mu.Unlock()
			return n, fmt.Errorf("read error after %d retries: %w", r.retryCount, err)
		}

		// Handle premature EOF
		if err == io.EOF && r.position < r.totalSize {
			if r.retryCount < r.config.MaxRetries {
				r.lastError = io.ErrUnexpectedEOF
				resumeErr := r.resumeFromPositionLocked()
				if resumeErr != nil {
					r.mu.Unlock()
					return n, fmt.Errorf("premature EOF at %d/%d bytes and resume failed: %w", r.position, r.totalSize, resumeErr)
				}
				if n > 0 {
					r.mu.Unlock()
					return n, nil
				}
				// Loop to try reading from new connection
				r.mu.Unlock()
				continue
			}
			r.mu.Unlock()
			return n, fmt.Errorf("premature EOF at %d/%d bytes after %d retries: %w", r.position, r.totalSize, r.retryCount, io.ErrUnexpectedEOF)
		}

		r.mu.Unlock()
		return n, err
	}
}

// resumeFromPositionLocked closes the current reader and opens a new one starting from the current position.
// Caller must hold r.mu lock. The lock is temporarily released during backoff wait and S3 request.
func (r *ResumableReader) resumeFromPositionLocked() error {
	r.retryCount++
	r.totalRetries++

	// Close current reader and log any errors (but don't fail on them)
	if r.currentReader != nil {
		if closeErr := r.currentReader.Close(); closeErr != nil {
			// Log but don't fail - we're already in error recovery
			r.config.Logger.V(1).Info("error closing reader during resume (non-fatal)",
				"error", closeErr,
				"bucket", r.bucket,
				"key", r.objectKey,
			)
		}
		r.currentReader = nil
	}

	// Calculate backoff with overflow protection
	var backoff time.Duration
	if r.retryCount <= 0 {
		backoff = r.config.InitialBackoff
	} else if r.retryCount > 30 {
		// Prevent overflow: 1<<30 is already > 1 billion
		backoff = r.config.MaxBackoff
	} else {
		backoff = r.config.InitialBackoff * time.Duration(1<<(r.retryCount-1))
	}
	if backoff > r.config.MaxBackoff {
		backoff = r.config.MaxBackoff
	}

	// Capture values while holding lock (before releasing for I/O)
	position := r.position
	totalSize := r.totalSize
	getOpts := r.getOpts

	// Calculate percent complete safely (avoid division by zero)
	var percentComplete float64
	if totalSize > 0 {
		percentComplete = float64(position) / float64(totalSize) * 100
	}

	r.config.Logger.Info("resuming S3 download",
		"bucket", r.bucket,
		"key", r.objectKey,
		"position", position,
		"totalSize", totalSize,
		"percentComplete", percentComplete,
		"retryCount", r.retryCount,
		"totalRetries", r.totalRetries,
		"backoffSeconds", backoff.Seconds(),
		"lastError", fmt.Sprintf("%v", r.lastError),
	)

	// Release lock during backoff wait and S3 request
	r.mu.Unlock()

	// Wait before retrying
	select {
	case <-time.After(backoff):
	case <-r.ctx.Done():
		r.mu.Lock()
		return r.ctx.Err()
	}

	// Create new request with Range header using captured values
	opts := getOpts
	opts.SetRange(position, totalSize-1)

	obj, err := r.client.GetObject(r.ctx, r.bucket, r.objectKey, opts)

	// Re-acquire lock before modifying state
	r.mu.Lock()

	// Check if closed while we were waiting/reconnecting (race with Close())
	if r.closed {
		if obj != nil {
			obj.Close() // Prevent resource leak
		}
		return errors.New("reader closed during resume")
	}

	if err != nil {
		return fmt.Errorf("error resuming from position %d: %w", position, err)
	}
	if obj == nil {
		return fmt.Errorf("nil object returned when resuming from position %d", position)
	}

	r.currentReader = obj

	r.config.Logger.Info("successfully resumed S3 download",
		"bucket", r.bucket,
		"key", r.objectKey,
		"position", r.position,
		"remainingBytes", r.totalSize-r.position,
	)

	return nil
}

// Close closes the reader.
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

// Position returns the current read position.
func (r *ResumableReader) Position() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.position
}

// TotalRetries returns the total number of retries that occurred.
func (r *ResumableReader) TotalRetries() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalRetries
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
