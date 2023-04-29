package replication

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
)

type mariadbClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	refResolver   *refresolver.RefResolver
	clientByIndex map[int]*mariadb.Client
}

func newMariaDBClientSet(mariadb *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver) (*mariadbClientSet, error) {
	if mariadb.Spec.Replication == nil {
		return nil, fmt.Errorf("'mariadb.spec.replication' is required to create a mariadbClientSet")
	}
	clientSet := &mariadbClientSet{
		mariadb:       mariadb,
		refResolver:   refResolver,
		clientByIndex: make(map[int]*mariadbclient.Client),
	}
	return clientSet, nil
}

func (c *mariadbClientSet) close() error {
	for i, rc := range c.clientByIndex {
		if err := rc.Close(); err != nil {
			return fmt.Errorf("error closing replica '%d' client: %v", i, err)
		}
	}
	return nil
}

func (c *mariadbClientSet) currentPrimaryClient(ctx context.Context) (*mariadbclient.Client, error) {
	if c.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' not set")
	}
	client, err := c.clientForIndex(ctx, *c.mariadb.Status.CurrentPrimaryPodIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting current primary client: %v", err)
	}
	return client, nil
}

func (c *mariadbClientSet) newPrimaryClient(ctx context.Context) (*mariadb.Client, error) {
	client, err := c.clientForIndex(ctx, c.mariadb.Spec.Replication.Primary.PodIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting new primary client: %v", err)
	}
	return client, nil
}

func (c *mariadbClientSet) clientForIndex(ctx context.Context, index int) (*mariadbclient.Client, error) {
	if err := c.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index. %v", err)
	}
	if c, ok := c.clientByIndex[index]; ok {
		return c, nil
	}
	client, err := mariadbclient.NewRootClientWithPodIndex(ctx, c.mariadb, c.refResolver, index)
	if err != nil {
		return nil, fmt.Errorf("error creating replica '%d' client: %v", index, err)
	}
	c.clientByIndex[index] = client
	return client, nil
}

func (c *mariadbClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.mariadb.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of MariaDB replicas bounds [0, %d]", index, c.mariadb.Spec.Replicas-1)
}
