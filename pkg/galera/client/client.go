package client

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	sqlClientSet "github.com/mariadb-operator/mariadb-operator/pkg/sqlset"
)

type GaleraClient struct {
	sqlClientSet *sqlClientSet.ClientSet
	opts         []sql.Opt
}

func NewGaleraClient(sqlClientSet *sqlClientSet.ClientSet, opts ...sql.Opt) *GaleraClient {
	return &GaleraClient{
		sqlClientSet: sqlClientSet,
		opts:         opts,
	}
}

func (g *GaleraClient) IsPodHealthy(ctx context.Context, podIndex int) (bool, error) {
	client, err := g.sqlClientSet.ClientForIndex(ctx, podIndex, g.opts...)
	if err != nil {
		return false, fmt.Errorf("error getting SQL client: %v", err)
	}

	status, err := client.GaleraClusterStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting cluster status: %v", err)
	}

	return status == "Primary", nil
}

func (g *GaleraClient) IsPodSynced(ctx context.Context, podIndex int) (bool, error) {
	healthy, err := g.IsPodHealthy(ctx, podIndex)
	if err != nil {
		return false, fmt.Errorf("error checking Pod health: %v", err)
	}
	if !healthy {
		return false, nil
	}

	client, err := g.sqlClientSet.ClientForIndex(ctx, podIndex, g.opts...)
	if err != nil {
		return false, fmt.Errorf("error getting SQL client: %v", err)
	}

	state, err := client.GaleraLocalState(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting local state: %v", err)
	}

	return state == "Synced", nil
}
