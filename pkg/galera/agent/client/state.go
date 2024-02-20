package client

import (
	"context"
	"net/http"

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

func (b *State) IsMariaDBInit(ctx context.Context) (bool, error) {
	res, err := b.client.Get(ctx, "/api/state/mariadb", nil)
	if err != nil {
		return false, err
	}
	if res.StatusCode == http.StatusOK {
		return true, nil
	}
	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, handleResponse(res, nil)
}
