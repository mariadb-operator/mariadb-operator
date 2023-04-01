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
	mariadb     *mariadbv1alpha1.MariaDB
	replClients map[int]*mariadb.Client
}

func newMariaDBClientSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	refResolver *refresolver.RefResolver) (*mariadbClientSet, error) {
	if mariadb.Spec.Replication == nil {
		return nil, fmt.Errorf("'mariadb.spec.replication' is required to create a mariadbClientSet")
	}

	clientSet := &mariadbClientSet{
		mariadb:     mariadb,
		replClients: make(map[int]*mariadbclient.Client, mariadb.Spec.Replicas),
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		client, err := mariadbclient.NewRootClientWithPodIndex(ctx, mariadb, refResolver, i)
		if err != nil {
			return nil, fmt.Errorf("error creating replica %d client: %v", i, err)
		}
		clientSet.replClients[i] = client
	}
	return clientSet, nil
}

func (c *mariadbClientSet) close() error {
	for i, rc := range c.replClients {
		if err := rc.Close(); err != nil {
			return fmt.Errorf("error closing replica %d: %v", i, err)
		}
	}
	return nil
}

func (c *mariadbClientSet) currentPrimaryClient() (*mariadbclient.Client, error) {
	if c.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' not set")
	}
	if rc, ok := c.replClients[*c.mariadb.Status.CurrentPrimaryPodIndex]; ok {
		return rc, nil
	}
	return nil, fmt.Errorf("current primary client not found, using index %d", *c.mariadb.Status.CurrentPrimaryPodIndex)
}

func (c *mariadbClientSet) newPrimaryClient() (*mariadb.Client, error) {
	if rc, ok := c.replClients[c.mariadb.Spec.Replication.PrimaryPodIndex]; ok {
		return rc, nil
	}
	return nil, fmt.Errorf("replica client not found, using index %d", c.mariadb.Spec.Replication.PrimaryPodIndex)
}

func (c *mariadbClientSet) replicaClient(replicaIndex int) (*mariadb.Client, error) {
	if rc, ok := c.replClients[replicaIndex]; ok {
		return rc, nil
	}
	return nil, fmt.Errorf("replica client not found, using index %d", replicaIndex)
}
