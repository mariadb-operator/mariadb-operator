package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

var defaultAdminUser = "admin"

type Client struct {
	User   *UserClient
	Server *ServerClient
}

func NewClient(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	httpClient, err := mdbhttp.NewClient(baseUrl, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %v", err)
	}
	return &Client{
		User: &UserClient{
			client: httpClient,
		},
		Server: &ServerClient{
			client: httpClient,
		},
	}, nil
}

func NewClientWithDefaultCredentials(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	opts = append(opts, mdbhttp.WithBasicAuth(defaultAdminUser, "mariadb"))
	return NewClient(baseUrl, opts...)
}

func handleResponse(res *http.Response, v interface{}) error {
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)

	if res.StatusCode >= 400 {
		var apiErr APIError
		if err := decoder.Decode(&apiErr); err != nil {
			return fmt.Errorf("error decoding body into error: %v", err)
		}
		return NewError(res.StatusCode, apiErr.Error())
	}

	if v == nil {
		return nil
	}
	if err := decoder.Decode(&v); err != nil {
		return fmt.Errorf("error decoding body: %v", err)
	}
	return nil
}
