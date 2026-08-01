package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/handler/replication"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

type Replication struct {
	client *mdbhttp.Client
}

func NewReplication(client *mdbhttp.Client) *Replication {
	return &Replication{
		client: client,
	}
}

func (r *Replication) GetGtid(ctx context.Context) (string, error) {
	gtidRes, err := r.GetGtidMeta(ctx)
	if err != nil {
		return "", err
	}
	return gtidRes.Gtid, nil
}

// GetGtidMeta returns the GTID together with the binary log coordinates it was derived from,
// enabling the caller to validate the GTID against the primary binary log.
func (r *Replication) GetGtidMeta(ctx context.Context) (*replication.GtidResponse, error) {
	res, err := r.client.Get(ctx, "/api/replication/gtid", nil, nil)
	if err != nil {
		return nil, err
	}
	var gtidRes replication.GtidResponse
	if err := handleResponse(res, &gtidRes); err != nil {
		return nil, err
	}
	return &gtidRes, nil
}
