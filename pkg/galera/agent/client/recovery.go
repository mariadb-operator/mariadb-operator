package client

import (
	"context"
	"net/http"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
)

type Recovery struct {
	*Client
}

func (r *Recovery) Enable(ctx context.Context) error {
	req, err := r.newRequestWithContext(ctx, http.MethodPut, "/api/recovery", nil)
	if err != nil {
		return err
	}
	return r.do(req, nil)
}

func (r *Recovery) Start(ctx context.Context) (*recovery.Bootstrap, error) {
	req, err := r.newRequestWithContext(ctx, http.MethodPost, "/api/recovery", nil)
	if err != nil {
		return nil, err
	}
	var bootstrap recovery.Bootstrap
	if err := r.do(req, &bootstrap); err != nil {
		return nil, err
	}
	return &bootstrap, nil
}

func (r *Recovery) Disable(ctx context.Context) error {
	req, err := r.newRequestWithContext(ctx, http.MethodDelete, "/api/recovery", nil)
	if err != nil {
		return err
	}
	return r.do(req, nil)
}
