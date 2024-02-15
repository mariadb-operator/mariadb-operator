package galera

import (
	"errors"
	"fmt"
	"sync"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/client"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
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
			mariadb.InternalServiceKey().Name,
		),
		ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Agent.Port,
	)
}
