package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type GaleraState struct {
	client *mdbhttp.Client
}

func NewGaleraState(client *mdbhttp.Client) *GaleraState {
	return &GaleraState{
		client: client,
	}
}

func (g *GaleraState) Get(ctx context.Context) (*recovery.GaleraState, error) {
	res, err := g.client.Get(ctx, "/api/galerastate", nil)
	if err != nil {
		return nil, err
	}
	var galeraState recovery.GaleraState
	if err := handleResponse(res, &galeraState); err != nil {
		return nil, err
	}
	return &galeraState, nil
}
