package galera

import (
	"errors"
	"fmt"
	"sync"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/galera/agent/client"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"k8s.io/utils/ptr"
)

type agentClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	clientOpts    []mdbhttp.Option
	clientByIndex map[int]*client.Client
	mux           *sync.Mutex
}

func newAgentClientSet(mariadb *mariadbv1alpha1.MariaDB, opts ...mdbhttp.Option) (*agentClientSet, error) {
	if !mariadb.IsGaleraEnabled() {
		return nil, errors.New("'mariadb.spec.galera.enabled' should be enabled to create an agent agentClientSet")
	}
	return &agentClientSet{
		mariadb:       mariadb,
		clientOpts:    opts,
		clientByIndex: make(map[int]*client.Client),
		mux:           &sync.Mutex{},
	}, nil
}

func (c *agentClientSet) clientForIndex(index int) (*client.Client, error) {
	if err := c.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index. %v", err)
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	if client, ok := c.clientByIndex[index]; ok {
		return client, nil
	}

	client, err := client.NewClient(baseUrl(c.mariadb, index), c.clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	c.clientByIndex[index] = client

	return client, nil
}

func (c *agentClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.mariadb.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of MariaDB replicas bounds [0, %d]", index, c.mariadb.Spec.Replicas-1)
}

func baseUrl(mariadb *mariadbv1alpha1.MariaDB, index int) string {
	scheme := "http"
	if mariadb.IsTLSEnabled() {
		scheme = "https"
	}
	return fmt.Sprintf(
		"%s://%s:%d",
		scheme,
		statefulset.PodFQDNWithService(
			mariadb.ObjectMeta,
			index,
			mariadb.InternalServiceKey().Name,
		),
		ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Agent.Port,
	)
}
