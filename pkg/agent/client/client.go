package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/errors"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
)

type Client struct {
	Galera      *Galera
	Replication *Replication
}

func NewClient(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	httpClient, err := mdbhttp.NewClient(baseUrl, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %v", err)
	}

	return &Client{
		Galera:      NewGalera(httpClient),
		Replication: NewReplication(httpClient),
	}, nil
}

func NewClientWithMariaDB(mariadb *mariadbv1alpha1.MariaDB, podIndex int, opts ...mdbhttp.Option) (*Client, error) {
	baseUrl, err := getAgentBaseUrl(mariadb, podIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting agent base URL: %v", err)
	}
	return NewClient(baseUrl, opts...)
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

func getAgentBaseUrl(mariadb *mariadbv1alpha1.MariaDB, index int) (string, error) {
	_, agent, err := mariadb.GetDataPlaneAgent()
	if err != nil {
		return "", fmt.Errorf("error getting agent: %v", err)
	}
	scheme := "http"
	if mariadb.IsTLSEnabled() {
		scheme = "https"
	}
	return fmt.Sprintf(
		"%s://%s:%d",
		scheme,
		statefulset.PodFQDNWithService(
			mariadb.ObjectMeta,
			index,
			mariadb.InternalServiceKey().Name,
		),
		agent.Port,
	), nil
}
