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
)

func (c *Client) NewRequestWithContext(ctx context.Context, method string, path string, body interface{},
	query map[string]string) (*http.Request, error) {

	baseUrl, err := c.buildUrl(path, query)
	if err != nil {
		return nil, fmt.Errorf("error building URL: %v", err)
	}

	if method == http.MethodGet {
		req, err := http.NewRequestWithContext(ctx, method, baseUrl.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error creating GET request: %v", err)
		}
		return req, nil
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error encoding body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	return http.NewRequestWithContext(ctx, method, baseUrl.String(), bodyReader)
}

func (c *Client) buildUrl(path string, query map[string]string) (*url.URL, error) {
	baseUrl := *c.baseUrl
	if c.version != "" {
		baseUrl.Path += fmt.Sprintf("/%s", c.version)
	}
	if !strings.HasSuffix(baseUrl.Path, "/") {
		baseUrl.Path += "/"
	}
	baseUrl.Path += strings.TrimPrefix(path, "/")

	if query != nil {
		q := baseUrl.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		baseUrl.RawQuery = q.Encode()
	}

	newUrl, err := url.Parse(baseUrl.String())
	if err != nil {
		return nil, err
	}
	return newUrl, nil
}
