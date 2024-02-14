package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type Recovery struct {
	client *mdbhttp.Client
}

func NewRecovery(client *mdbhttp.Client) *Recovery {
	return &Recovery{
		client: client,
	}
}

func (r *Recovery) Enable(ctx context.Context) error {
	res, err := r.client.Put(ctx, "/api/recovery", nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (r *Recovery) Start(ctx context.Context) (*recovery.Bootstrap, error) {
	res, err := r.client.Post(ctx, "/api/recovery", nil, nil)
	if err != nil {
		return nil, err
	}
	var bootstrap recovery.Bootstrap
	if err := handleResponse(res, &bootstrap); err != nil {
		return nil, err
	}
	return &bootstrap, nil
}

func (r *Recovery) Disable(ctx context.Context) error {
	res, err := r.client.Delete(ctx, "/api/recovery", nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
