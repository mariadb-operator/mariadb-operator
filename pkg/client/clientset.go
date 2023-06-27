package client

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
)

type ClientSet struct {
	Mariadb       *mariadbv1alpha1.MariaDB
	refResolver   *refresolver.RefResolver
	clientByIndex map[int]*Client
}

func NewClientSet(mariadb *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver) *ClientSet {
	return &ClientSet{
		Mariadb:       mariadb,
		refResolver:   refResolver,
		clientByIndex: make(map[int]*Client),
	}
}

func (c *ClientSet) Close() error {
	for i, rc := range c.clientByIndex {
		if err := rc.Close(); err != nil {
			return fmt.Errorf("error closing replica '%d' client: %v", i, err)
		}
	}
	return nil
}

func (c *ClientSet) ClientForIndex(ctx context.Context, index int, clientOpts ...Opt) (*Client, error) {
	if err := c.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index. %v", err)
	}
	if c, ok := c.clientByIndex[index]; ok {
		return c, nil
	}
	client, err := NewInternalClientWithPodIndex(ctx, c.Mariadb, c.refResolver, index, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating replica '%d' client: %v", index, err)
	}
	c.clientByIndex[index] = client
	return client, nil
}

func (c *ClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.Mariadb.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of MariaDB replicas bounds [0, %d]", index, c.Mariadb.Spec.Replicas-1)
}
