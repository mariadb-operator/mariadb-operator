package galera

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controllers/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
)

type agentClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	clientOpts    []client.Option
	clientByIndex map[int]*client.Client
	mux           *sync.Mutex
}

func newAgentClientSet(mariadb *mariadbv1alpha1.MariaDB, clientOpts ...client.Option) (*agentClientSet, error) {
	if mariadb.Spec.Galera == nil {
		return nil, errors.New("'mariadb.spec.galera' is required to create an agent agentClientSet")
	}
	opts := clientOpts
	if opts == nil {
		opts = []client.Option{
			client.WithTimeout(5 * time.Second),
		}
	}

	return &agentClientSet{
		mariadb:       mariadb,
		clientOpts:    opts,
		clientByIndex: make(map[int]*client.Client),
		mux:           &sync.Mutex{},
	}, nil
}

func (a *agentClientSet) clientForIndex(index int) (*client.Client, error) {
	if err := a.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index. %v", err)
	}
	a.mux.Lock()
	defer a.mux.Unlock()
	if c, ok := a.clientByIndex[index]; ok {
		return c, nil
	}

	c, err := client.NewClient(baseUrl(a.mariadb, index), a.clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	a.clientByIndex[index] = c

	return c, nil
}

func (c *agentClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.mariadb.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of MariaDB replicas bounds [0, %d]", index, c.mariadb.Spec.Replicas-1)
}

func baseUrl(mariadb *mariadbv1alpha1.MariaDB, index int) string {
	return fmt.Sprintf(
		"http://%s:%d",
		statefulset.PodFQDNWithService(
			mariadb.ObjectMeta,
			index,
			ctrlresources.InternalServiceKey(mariadb).Name,
		),
		mariadb.Spec.Galera.Agent.Port,
	)
}
