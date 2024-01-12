package client

import (
	"context"
	"fmt"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type ServerAttributes struct {
	Address  string
	Port     int
	Protocol string
}

type Server struct {
	client *mdbhttp.Client
}

func (s *Server) Create(ctx context.Context, name string, attrs ServerAttributes) error {
	payload := &Payload[ServerAttributes]{
		Data: PayloadData[ServerAttributes]{
			ID:         name,
			Type:       ObjectTypeServers,
			Attributes: attrs,
		},
	}
	res, err := s.client.Post(ctx, "servers", payload, nil)
	if err != nil {
		return fmt.Errorf("error creating server: %v", err)
	}
	return handleResponse(res, nil)
}
