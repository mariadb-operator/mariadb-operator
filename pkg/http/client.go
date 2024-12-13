package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

var defaultTimeout = 10 * time.Second

type Option func(*Opts) error

type Opts struct {
	httpClient *http.Client
	headers    map[string]string
	version    string
	logger     *logr.Logger

	tlsEnabled bool
	tlsCACert  []byte
	tlsCert    []byte
	tlsKey     []byte
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(opts *Opts) error {
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		opts.httpClient = httpClient
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(opts *Opts) error {
		if timeout == 0 {
			timeout = defaultTimeout
		}
		opts.httpClient.Timeout = timeout
		return nil
	}
}

func WithBasicAuth(username, password string) Option {
	return func(opts *Opts) error {
		raw := fmt.Sprintf("%s:%s", username, password)
		encoded := b64.StdEncoding.EncodeToString([]byte(raw))
		opts.headers["Authorization"] = fmt.Sprintf("Basic %s", encoded)
		return nil
	}
}

func WithKubernetesAuth(serviceAccountPath string) Option {
	return func(opts *Opts) error {
		bytes, err := os.ReadFile(serviceAccountPath)
		if err != nil {
			return fmt.Errorf("error getting Kubernetes auth header: error reading '%s': %v", serviceAccountPath, err)
		}
		opts.headers["Authorization"] = fmt.Sprintf("Bearer %s", string(bytes))
		return nil
	}
}

func WithVersion(version string) Option {
	return func(opts *Opts) error {
		opts.version = strings.TrimPrefix(version, "/")
		return nil
	}
}

func WithLogger(logger *logr.Logger) Option {
	return func(opts *Opts) error {
		opts.logger = logger
		return nil
	}
}

func WithTLSEnabled(tlsEnabled bool) Option {
	return func(opts *Opts) error {
		opts.tlsEnabled = tlsEnabled
		return nil
	}
}

func WithTLSCA(tlsCACert []byte) Option {
	return func(opts *Opts) error {
		opts.tlsCACert = tlsCACert
		return nil
	}
}

func WithTLSCert(tlsCert []byte) Option {
	return func(opts *Opts) error {
		opts.tlsCert = tlsCert
		return nil
	}
}

func WithTLSKey(tlsKey []byte) Option {
	return func(opts *Opts) error {
		opts.tlsKey = tlsKey
		return nil
	}
}

type Client struct {
	baseUrl    *url.URL
	httpClient *http.Client
	headers    map[string]string
	version    string
	logger     *logr.Logger
}

func NewClient(baseUrl string, opts ...Option) (*Client, error) {
	url, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %v", err)
	}

	clientOpts := Opts{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		headers: make(map[string]string, 0),
	}
	for _, setOpt := range opts {
		if err := setOpt(&clientOpts); err != nil {
			return nil, err
		}
	}

	client := &Client{
		baseUrl:    url,
		httpClient: clientOpts.httpClient,
		headers:    clientOpts.headers,
		version:    clientOpts.version,
		logger:     clientOpts.logger,
	}

	transport, err := client.getTransport(&clientOpts)
	if err != nil {
		return nil, fmt.Errorf("error getting transport: %v", err)
	}
	client.httpClient.Transport = NewHeadersTransport(transport, client.headers)
	return client, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if err := c.logRequest(req); err != nil {
		return nil, fmt.Errorf("error logging request: %v", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if err := c.logResponse(req, res); err != nil {
		return nil, fmt.Errorf("error logging response: %v", err)
	}
	return res, nil
}

func (c *Client) Request(ctx context.Context, method string, path string, body interface{},
	query map[string]string) (*http.Response, error) {
	req, err := c.NewRequestWithContext(ctx, method, path, body, query)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *Client) Get(ctx context.Context, path string, query map[string]string) (*http.Response, error) {
	req, err := c.NewRequestWithContext(ctx, http.MethodGet, path, nil, query)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *Client) Post(ctx context.Context, path string, body interface{}, query map[string]string) (*http.Response, error) {
	return c.Request(ctx, http.MethodPost, path, body, query)
}

func (c *Client) Put(ctx context.Context, path string, body interface{}, query map[string]string) (*http.Response, error) {
	return c.Request(ctx, http.MethodPut, path, body, query)
}

func (c *Client) Patch(ctx context.Context, path string, body interface{}, query map[string]string) (*http.Response, error) {
	return c.Request(ctx, http.MethodPatch, path, body, query)
}

func (c *Client) Delete(ctx context.Context, path string, body interface{}, query map[string]string) (*http.Response, error) {
	return c.Request(ctx, http.MethodDelete, path, body, query)
}

func (c *Client) logRequest(req *http.Request) error {
	c.logInfo("Request", "method", req.Method, "url", req.URL.String())
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.logDebug("Request body", "body", string(bodyBytes))
	}
	return nil
}

func (c *Client) logResponse(req *http.Request, res *http.Response) error {
	c.logInfo("Response", "method", req.Method, "url", req.URL.String(), "status-code", res.StatusCode)
	if res.Body != nil {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.logDebug("Response body", "body", string(bodyBytes))
	}
	return nil
}

func (c *Client) logInfo(msg string, kv ...interface{}) {
	if c.logger == nil {
		return
	}
	c.logger.Info(msg, kv...)
}

func (c *Client) logDebug(msg string, kv ...interface{}) {
	if c.logger == nil {
		return
	}
	c.logger.V(1).Info(msg, kv...)
}

func (c *Client) getTransport(opts *Opts) (http.RoundTripper, error) {
	if !opts.tlsEnabled {
		return http.DefaultTransport, nil
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(opts.tlsCACert); !ok {
		return nil, errors.New("unable to add CA cert to pool")
	}

	cert, err := tls.X509KeyPair(opts.tlsCert, opts.tlsKey)
	if err != nil {
		return nil, fmt.Errorf("error parsing x509 keypair: %v", err)
	}

	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:            caCertPool,
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: false,
		},
	}, nil
}
