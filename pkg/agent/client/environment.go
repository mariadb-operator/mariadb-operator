package client

import (
	"context"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

type Environment struct {
	client *mdbhttp.Client
}

func NewEnvironment(client *mdbhttp.Client) *Environment {
	return &Environment{
		client: client,
	}
}

func (e *Environment) SetValue(ctx context.Context, key, value string) error {
	body := struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{
		Key:   key,
		Value: value,
	}
	res, err := e.client.Put(ctx, "/api/environment", body, nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
