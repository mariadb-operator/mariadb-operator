package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type Client struct {
	Bootstrap *Bootstrap
	State     *State
	Recovery  *Recovery
}

func NewClient(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	httpClient, err := mdbhttp.NewClient(baseUrl, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %v", err)
	}

	return &Client{
		Bootstrap: NewBootstrap(httpClient),
		State:     NewState(httpClient),
		Recovery:  NewRecovery(httpClient),
	}, nil
}

func handleResponse(res *http.Response, v interface{}) error {
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)

	if res.StatusCode >= 400 {
		var apiErr errors.APIError
		if err := decoder.Decode(&apiErr); err != nil {
			return fmt.Errorf("error decoding body into error: %v", err)
		}
		return errors.NewError(res.StatusCode, apiErr.Error())
	}

	if v == nil {
		return nil
	}
	if err := decoder.Decode(&v); err != nil {
		return fmt.Errorf("error decoding body: %v", err)
	}
	return nil
}
