package client

import (
	"context"
	"net/http"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/galera/recovery"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
)

type Galera struct {
	client *mdbhttp.Client
}

func NewGalera(client *mdbhttp.Client) *Galera {
	return &Galera{
		client: client,
	}
}

func (g *Galera) Health(ctx context.Context) (bool, error) {
	res, err := g.client.Get(ctx, "/health", nil, nil)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK, nil
}

func (g *Galera) GetState(ctx context.Context) (*recovery.GaleraState, error) {
	res, err := g.client.Get(ctx, "/api/galera/state", nil, nil)
	if err != nil {
		return nil, err
	}
	var galeraState recovery.GaleraState
	if err := handleResponse(res, &galeraState); err != nil {
		return nil, err
	}
	return &galeraState, nil
}

func (g *Galera) IsBootstrapEnabled(ctx context.Context) (bool, error) {
	res, err := g.client.Get(ctx, "/api/galera/bootstrap", nil, nil)
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

func (g *Galera) EnableBootstrap(ctx context.Context, bootstrap *recovery.Bootstrap) error {
	res, err := g.client.Put(ctx, "/api/galera/bootstrap", bootstrap, nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (g *Galera) DisableBootstrap(ctx context.Context) error {
	res, err := g.client.Delete(ctx, "/api/galera/bootstrap", nil, nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
