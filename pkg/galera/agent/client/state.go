package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type State struct {
	client *mdbhttp.Client
}

func NewState(client *mdbhttp.Client) *State {
	return &State{
		client: client,
	}
}

func (g *State) GetGaleraState(ctx context.Context) (*recovery.GaleraState, error) {
	res, err := g.client.Get(ctx, "/api/state/galera", nil)
	if err != nil {
		return nil, err
	}
	var galeraState recovery.GaleraState
	if err := handleResponse(res, &galeraState); err != nil {
		return nil, err
	}
	return &galeraState, nil
}
