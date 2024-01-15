package client

import (
	"context"
	"errors"
	"strings"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

var ErrMasterServerNotFound = errors.New("master server not found")

type ServerParameters struct {
	Address  string `json:"address"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

type ServerAttributes struct {
	State      string           `json:"state,omitempty"`
	Parameters ServerParameters `json:"parameters"`
}

func (s *ServerAttributes) IsMaster() bool {
	return strings.Contains(s.State, "Master")
}

type ServerClient struct {
	ReadClient[ServerAttributes]
	client *mdbhttp.Client
}

func NewServerClient(client *mdbhttp.Client) *ServerClient {
	return &ServerClient{
		ReadClient: NewListClient[ServerAttributes](client, "servers"),
		client:     client,
	}
}

func (s *ServerClient) GetMaster(ctx context.Context) (string, error) {
	servers, err := s.List(ctx)
	if err != nil {
		return "", err
	}
	for _, srv := range servers {
		if srv.Attributes.IsMaster() {
			return srv.ID, nil
		}
	}
	return "", ErrMasterServerNotFound
}

func (s *ServerClient) Create(ctx context.Context, name string, params ServerParameters) error {
	object := &Object[ServerAttributes]{
		Data: Data[ServerAttributes]{
			ID:   name,
			Type: ObjectTypeServers,
			Attributes: ServerAttributes{
				Parameters: params,
			},
		},
	}
	res, err := s.client.Post(ctx, "servers", object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
