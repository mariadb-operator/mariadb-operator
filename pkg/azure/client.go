package azure

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"k8s.io/utils/ptr"
)

type AzBlobOpts struct {
	// Authentication Opts
	ServiceURL  string
	AccountName string
	AccountKey  string

	Prefix              string // A directory prefix... Perform All operations under here
	AllowNestedPrefixes bool

	// TLS Opts
	TLSEnabled    bool
	TLSCACert     []byte
	TLSCACertPath string
}

type AzBlobOpt func(o *AzBlobOpts)

func WithServiceURL(serviceURL string) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.ServiceURL = serviceURL
	}
}

func WithPrefix(prefix string) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.Prefix = prefix
	}
}

func WithAllowNestedPrefixes(allowNestedPrefixes bool) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.AllowNestedPrefixes = allowNestedPrefixes
	}
}

func WithAccountName(accountName string) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.AccountName = accountName
	}
}

func WithAccountKey(accountKey string) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.AccountKey = accountKey
	}
}

func WithTLSEnabled(tlsEnabled bool) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.TLSEnabled = tlsEnabled
	}
}

func WithTLSCACertPath(tlsCACertPath string) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.TLSCACertPath = tlsCACertPath
	}
}

func WithTLSCACertBytes(tlsCACert []byte) AzBlobOpt {
	return func(o *AzBlobOpts) {
		o.TLSCACert = tlsCACert
	}
}

type AzBlobClient struct {
	*azblob.Client

	Opts          *AzBlobOpts
	ContainerName string

	// Local FS Opts
	BasePath string
}

// NewAzBlobClient creates a Azure Blob Storage Client for a single container.
// basePath is used for local FS operations
// containerName is equivalent to `bucket` in s3
// serviceURL must be the full url: `https://%s.blob.core.windows.net/` where `%s` is the containerName
func NewAzBlobClient(basePath, containerName, serviceURL string, azOpts ...AzBlobOpt) (*AzBlobClient, error) {
	opts := &AzBlobOpts{
		ServiceURL: serviceURL,
	}
	for _, setOpt := range azOpts {
		setOpt(opts)
	}

	clientOptions, err := getClientOptions(opts)
	if err != nil {
		return nil, err
	}

	var client *azblob.Client

	if opts.AccountKey != "" && opts.AccountName != "" {
		cred, err := azblob.NewSharedKeyCredential(opts.AccountName, opts.AccountKey)
		if err != nil {
			return nil, err
		}

		client, err = azblob.NewClientWithSharedKeyCredential(opts.ServiceURL, cred, clientOptions)
		if err != nil {
			return nil, fmt.Errorf("error creating new client with shared key credentials: %w", err)
		}

	} else {
		tokenCred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}

		client, err = azblob.NewClient(opts.ServiceURL, tokenCred, clientOptions)
		if err != nil {
			return nil, fmt.Errorf("error creating new client with default credentials: %w", err)
		}
	}

	return &AzBlobClient{
		Client:        client,
		ContainerName: containerName,
		Opts:          opts,
		BasePath:      basePath,
	}, nil
}

// Blob Storage Interop

// PutObjectWithOptions will upload the given reader to Azure
// `size` is ignored and is passed to satisfy the interface
func (c *AzBlobClient) PutObjectWithOptions(ctx context.Context, fileName string, reader io.Reader, size int64) error {
	_, err := c.UploadStream(ctx, c.ContainerName, c.PrefixedFileName(fileName), reader, nil)

	return err
}

func (c *AzBlobClient) FPutObjectWithOptions(ctx context.Context, fileName string) error {
	file, err := os.Open(c.getFilePath(fileName))
	if err != nil {
		return err
	}
	defer file.Close()

	return c.PutObjectWithOptions(ctx, fileName, file, 0)
}

func (c *AzBlobClient) GetObjectWithOptions(ctx context.Context, fileName string) (io.ReadCloser, error) {
	resp, err := c.DownloadStream(ctx, c.ContainerName, c.PrefixedFileName(fileName), nil)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (c *AzBlobClient) FGetObjectWithOptions(ctx context.Context, fileName string) error {
	rc, err := c.GetObjectWithOptions(
		ctx,
		fileName,
	)
	if err != nil {
		return err
	}
	defer rc.Close()

	filePath := c.getFilePath(fileName)
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, rc) // This is more efficient than the example with ReadAll
	return err
}

func (c *AzBlobClient) RemoveWithOptions(ctx context.Context, fileName string) error {
	_, err := c.DeleteBlob(ctx, c.ContainerName, c.PrefixedFileName(fileName), nil)

	return err
}

func (c *AzBlobClient) Exists(ctx context.Context, fileName string) (bool, error) {
	_, err := c.ServiceClient().
		NewContainerClient(c.ContainerName).
		NewBlobClient(c.PrefixedFileName(fileName)).GetProperties(ctx, nil)
	if err != nil {
		// Ref: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azcore#ResponseError
		return false, ignoreNotFound(err)
	}

	return true, nil
}

func (c *AzBlobClient) PrefixedFileName(fileName string) string {
	if c.Opts.AllowNestedPrefixes {
		return c.GetPrefix() + fileName
	}
	return c.GetPrefix() + filepath.Base(fileName)
}

func (c *AzBlobClient) UnprefixedFilename(fileName string) string {
	return strings.TrimPrefix(filepath.Base(fileName), c.GetPrefix())
}

func (c *AzBlobClient) GetPrefix() string {
	if c.Opts.Prefix == "" || c.Opts.Prefix == "/" {
		return "" // object store doesn't use slash for root path
	}
	if !strings.HasSuffix(c.Opts.Prefix, "/") {
		return c.Opts.Prefix + "/" // ending slash is required for avoiding matching like "foo/" and "foobar/" with prefix "foo"
	}
	return c.Opts.Prefix
}

func (c *AzBlobClient) ListObjectsWithOptions(ctx context.Context) ([]string, error) {
	pager := c.NewListBlobsFlatPager(c.ContainerName, &container.ListBlobsFlatOptions{
		Prefix: ptr.To(c.GetPrefix()),
	})
	items := make([]string, 0)

	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return []string{}, err
		}

		for _, v := range resp.Segment.BlobItems {
			items = append(items, *v.Name)
		}
	}
	return items, nil
}

// IsAuthenticated will do a simple property check on the container to validate authentication
// RESPONSE 403: 403 Server failed to authenticate the request....
func (c *AzBlobClient) IsAuthenticated() bool {
	_, err := c.ServiceClient().
		NewContainerClient(c.ContainerName).GetProperties(context.Background(), nil)

	if err != nil {
		code := getStatusCodeFromErr(err)
		return code != http.StatusForbidden
	}

	return true
}

func (c *AzBlobClient) getFilePath(fileName string) string {
	if filepath.IsAbs(fileName) {
		return fileName
	}
	return filepath.Join(c.BasePath, fileName)
}

// ===============
func getStatusCodeFromErr(err error) int {
	if err == nil {
		return 0
	}

	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode
	}

	return 0
}

// ignoreNotFound will return nil if the error is 404 not found
func ignoreNotFound(err error) error {
	if getStatusCodeFromErr(err) == http.StatusNotFound {
		return nil
	}

	return err
}

func getClientOptions(opts *AzBlobOpts) (*azblob.ClientOptions, error) {
	if !opts.TLSEnabled {
		return &azblob.ClientOptions{}, nil
	}

	transport, err := getTransport(opts)
	if err != nil {
		return nil, fmt.Errorf("error creating azure blob transport: %w", err)
	}

	return &azblob.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Transport: &http.Client{
				Transport: transport,
			},
			Telemetry: policy.TelemetryOptions{
				Disabled: true,
			},
			Retry: policy.RetryOptions{},
			// Cloud: cloud.Configuration{}, //@TODO: if required, we can support this too,
		},
	}, nil
}

func getTransport(opts *AzBlobOpts) (http.RoundTripper, error) {
	if !opts.TLSEnabled {
		return http.DefaultTransport, nil
	}

	certBytes := opts.TLSCACert
	if opts.TLSCACertPath != "" {
		caBytes, err := os.ReadFile(opts.TLSCACertPath)
		if err != nil {
			return nil, fmt.Errorf("error reading CA cert: %v", err)
		}

		certBytes = caBytes
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
		return nil, errors.New("unable to add CA cert to pool")
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: false,
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}, nil
}
