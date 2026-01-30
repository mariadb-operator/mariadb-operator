package client

import (
	"context"
	"errors"
	"fmt"
	"sync"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
)

type ClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	clientOpts    []mdbhttp.Option
	clientByIndex map[int]*Client
	mux           *sync.Mutex
}

func NewClientSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, env *environment.OperatorEnv,
	refResolver *refresolver.RefResolver, opts ...mdbhttp.Option) (*ClientSet, error) {
	if !mariadb.IsHAEnabled() {
		return nil, errors.New("HA should be enabled to create an agent ClientSet")
	}
	clientOpts, err := getClientOpts(ctx, mariadb, env, refResolver)
	if err != nil {
		return nil, fmt.Errorf("error getting client options: %v", err)
	}
	clientOpts = append(clientOpts, opts...)

	return &ClientSet{
		mariadb:       mariadb,
		clientOpts:    clientOpts,
		clientByIndex: make(map[int]*Client),
		mux:           &sync.Mutex{},
	}, nil
}

func (c *ClientSet) ClientForIndex(index int) (*Client, error) {
	if err := c.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index: %v", err)
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	if client, ok := c.clientByIndex[index]; ok {
		return client, nil
	}

	baseUrl, err := getAgentBaseUrl(c.mariadb, index)
	if err != nil {
		return nil, fmt.Errorf("error getting base URL: %v", err)
	}
	client, err := NewClient(baseUrl, c.clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	c.clientByIndex[index] = client

	return client, nil
}

func (c *ClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.mariadb.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of MariaDB replicas bounds [0, %d]", index, c.mariadb.Spec.Replicas-1)
}
