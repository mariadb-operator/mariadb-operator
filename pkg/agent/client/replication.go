package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/handler/replication"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
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
	res, err := r.client.Get(ctx, "/api/replication/gtid", nil, nil)
	if err != nil {
		return "", err
	}
	var gtidRes replication.GtidResponse
	if err := handleResponse(res, &gtidRes); err != nil {
		return "", err
	}
	return gtidRes.Gtid, nil
}
