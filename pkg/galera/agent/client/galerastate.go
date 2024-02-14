package client

import (
	"context"
	"net/http"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
)

type GaleraState struct {
	*Client
}

func (g *GaleraState) Get(ctx context.Context) (*recovery.GaleraState, error) {
	req, err := g.newRequestWithContext(ctx, http.MethodGet, "/api/galerastate", nil)
	if err != nil {
		return nil, err
	}
	var galeraState recovery.GaleraState
	if err := g.do(req, &galeraState); err != nil {
		return nil, err
	}
	return &galeraState, nil
}
