package client

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
)

func IsPodHealthy(ctx context.Context, sqlClient *sql.Client) (bool, error) {
	status, err := sqlClient.GaleraClusterStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting cluster status: %v", err)
	}

	return status == "Primary", nil
}

var (
	GaleraStateSynced string = "Synced"
	GaleraStateDonor  string = "Donor/Desynced"
)

func IsPodSynced(ctx context.Context, sqlClient *sql.Client) (bool, error) {
	healthy, err := IsPodHealthy(ctx, sqlClient)
	if err != nil {
		return false, fmt.Errorf("error checking Pod health: %v", err)
	}
	if !healthy {
		return false, nil
	}

	state, err := sqlClient.GaleraLocalState(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting local state: %v", err)
	}

	return state == GaleraStateSynced, nil
}
