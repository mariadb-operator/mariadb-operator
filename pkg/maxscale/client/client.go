package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

var defaultAdminUser = "admin"

type Client struct {
	User     *UserClient
	Server   *ServerClient
	Monitor  *MonitorClient
	Service  *ServiceClient
	Listener *ListenerClient
	MaxScale *MaxScaleClient
}

func NewClient(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	httpClient, err := mdbhttp.NewClient(baseUrl, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %v", err)
	}
	return &Client{
		User:     NewUserClient(httpClient),
		Server:   NewServerClient(httpClient),
		Monitor:  NewMonitorClient(httpClient),
		Service:  NewServiceClient(httpClient),
		Listener: NewListenerClient(httpClient),
		MaxScale: NewMaxScaleClient(httpClient),
	}, nil
}

func NewClientWithDefaultCredentials(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	opts = append(opts, mdbhttp.WithBasicAuth(defaultAdminUser, "mariadb"))
	return NewClient(baseUrl, opts...)
}

func handleResponse(res *http.Response, v interface{}) error {
	defer res.Body.Close()
	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %v", err)
	}

	if res.StatusCode >= 400 {
		if len(bytes) == 0 {
			return NewError(res.StatusCode, res.Status)
		}
		var apiErr APIError
		if err := json.Unmarshal(bytes, &apiErr); err != nil {
			return fmt.Errorf("error decoding body into error: %v", err)
		}
		return NewError(res.StatusCode, apiErr.Error())
	}

	if v == nil {
		return nil
	}
	if err := json.Unmarshal(bytes, &v); err != nil {
		return fmt.Errorf("error decoding body: %v", err)
	}
	return nil
}
