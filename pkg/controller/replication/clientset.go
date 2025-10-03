package replication

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"k8s.io/utils/ptr"
)

type ReplicationClientSet struct {
	*sqlClient.ClientSet
}

func NewReplicationClientSet(mariadb *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver) (*ReplicationClientSet, error) {
	if !mariadb.IsReplicationEnabled() {
		return nil, errors.New("'mariadb.spec.replication' is required to create a replicationClientSet")
	}
	return &ReplicationClientSet{
		ClientSet: sqlClient.NewClientSet(mariadb, refResolver),
	}, nil
}

func (c *ReplicationClientSet) close() error {
	return c.Close()
}

func (c *ReplicationClientSet) clientForIndex(ctx context.Context, index int, clientOpts ...sqlClient.Opt) (*sqlClient.Client, error) {
	return c.ClientForIndex(ctx, index, clientOpts...)
}

func (c *ReplicationClientSet) currentPrimaryClient(ctx context.Context, clientOpts ...sqlClient.Opt) (*sqlClient.Client, error) {
	if c.Mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	client, err := c.ClientForIndex(ctx, *c.Mariadb.Status.CurrentPrimaryPodIndex, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error getting current primary client: %v", err)
	}
	return client, nil
}

func (c *ReplicationClientSet) newPrimaryClient(ctx context.Context, clientOpts ...sqlClient.Opt) (*sqlClient.Client, error) {
	replication := ptr.Deref(c.Mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	client, err := c.ClientForIndex(ctx, *replication.Primary.PodIndex, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error getting new primary client: %v", err)
	}
	return client, nil
}
