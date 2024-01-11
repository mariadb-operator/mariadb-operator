package client

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		c.httpClient = httpClient
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		c.httpClient.Timeout = timeout
	}
}

func WithAdminCredentials(username, password string) Option {
	return func(c *Client) {
		raw := fmt.Sprintf("%s:%s", username, password)
		encoded := b64.StdEncoding.EncodeToString([]byte(raw))
		c.headers["Authorization"] = fmt.Sprintf("Basic %s", encoded)
	}
}

func WithVersion(version string) Option {
	return func(c *Client) {
		c.version = strings.TrimPrefix(version, "/")
	}
}

type Client struct {
	baseUrl    *url.URL
	httpClient *http.Client
	headers    map[string]string
	version    string
}

func NewClient(baseUrl string, opts ...Option) (*Client, error) {
	url, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %v", err)
	}
	client := &Client{
		baseUrl:    url,
		httpClient: http.DefaultClient,
		headers:    make(map[string]string, 0),
	}
	for _, setOpt := range opts {
		setOpt(client)
	}
	client.httpClient.Transport = &HeadersTransport{
		RoundTripper: client.httpClient.Transport,
		headers:      client.headers,
	}
	return client, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
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

type HeadersTransport struct {
	RoundTripper http.RoundTripper
	headers      map[string]string
}

func (t *HeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
	}
	return t.RoundTripper.RoundTrip(req)
}
