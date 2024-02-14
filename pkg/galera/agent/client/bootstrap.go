package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type Bootstrap struct {
	client *mdbhttp.Client
}

func NewBootstrap(client *mdbhttp.Client) *Bootstrap {
	return &Bootstrap{
		client: client,
	}
}

func (b *Bootstrap) Enable(ctx context.Context, bootstrap *recovery.Bootstrap) error {
	res, err := b.client.Put(ctx, "/api/bootstrap", bootstrap, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (b *Bootstrap) Disable(ctx context.Context) error {
	res, err := b.client.Delete(ctx, "/api/bootstrap", nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
