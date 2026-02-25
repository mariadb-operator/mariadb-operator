package client

import (
	"context"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/handler/gtid"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
)

type Gtid struct {
	client *mdbhttp.Client
}

func NewGtid(client *mdbhttp.Client) *Gtid {
	return &Gtid{
		client: client,
	}
}

func (r *Gtid) GetGtid(ctx context.Context) (string, error) {
	res, err := r.client.Get(ctx, "/api/gtid", nil, nil)
	if err != nil {
		return "", err
	}
	var gtidRes gtid.GtidResponse
	if err := handleResponse(res, &gtidRes); err != nil {
		return "", err
	}
	return gtidRes.Gtid, nil
}
