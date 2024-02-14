package client

import (
	"context"
	"net/http"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
)

type Bootstrap struct {
	*Client
}

func (b *Bootstrap) Enable(ctx context.Context, bootstrap *recovery.Bootstrap) error {
	req, err := b.newRequestWithContext(ctx, http.MethodPut, "/api/bootstrap", bootstrap)
	if err != nil {
		return err
	}
	return b.do(req, nil)
}

func (b *Bootstrap) Disable(ctx context.Context) error {
	req, err := b.newRequestWithContext(ctx, http.MethodDelete, "/api/bootstrap", nil)
	if err != nil {
		return err
	}
	return b.do(req, nil)
}
