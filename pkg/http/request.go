package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	mdbreflect "github.com/mariadb-operator/mariadb-operator/pkg/reflect"
)

func (c *Client) NewRequestWithContext(ctx context.Context, method string, path string, body interface{},
	query map[string]string) (*http.Request, error) {

	baseURL, err := c.buildURL(path, query)
	if err != nil {
		return nil, fmt.Errorf("error building URL: %v", err)
	}

	if method == http.MethodGet {
		req, err := http.NewRequestWithContext(ctx, method, baseURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error creating GET request: %v", err)
		}
		return req, nil
	}

	var bodyReader io.Reader
	if !mdbreflect.IsNil(body) {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error encoding body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	return http.NewRequestWithContext(ctx, method, baseURL.String(), bodyReader)
}

func (c *Client) buildURL(path string, query map[string]string) (*url.URL, error) {
	baseURL := *c.baseURL
	if c.version != "" {
		baseURL.Path += fmt.Sprintf("/%s", c.version)
	}
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}
	baseURL.Path += strings.TrimPrefix(path, "/")

	if query != nil {
		q := baseURL.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		baseURL.RawQuery = q.Encode()
	}

	newURL, err := url.Parse(baseURL.String())
	if err != nil {
		return nil, err
	}
	return newURL, nil
}
