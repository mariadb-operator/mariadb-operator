package galera

import (
	"errors"
	"fmt"
	"time"

	"github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
)

type agentClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	clientByIndex map[int]*client.Client
}

func newAgentClientSet(mariadb *mariadbv1alpha1.MariaDB) (*agentClientSet, error) {
	if mariadb.Spec.Galera == nil {
		return nil, errors.New("'mariadb.spec.galera' is required to create an agent agentClientSet")
	}
	return &agentClientSet{
		mariadb:       mariadb,
		clientByIndex: make(map[int]*client.Client),
	}, nil
}

func (a *agentClientSet) clientForIndex(index int) (*client.Client, error) {
	if err := a.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index. %v", err)
	}
	if c, ok := a.clientByIndex[index]; ok {
		return c, nil
	}

	c, err := client.NewClient(
		a.baseUrl(a.mariadb, index),
		// TODO: expose to user via CRD
		client.WithTimeout(60*time.Second),
	)
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

func (c *agentClientSet) baseUrl(mariadb *mariadbv1alpha1.MariaDB, index int) string {
	return fmt.Sprintf(
		"http://%s:%d",
		statefulset.PodFQDNWithService(mariadb.ObjectMeta, index, galeraresources.ServiceKey(mariadb).Name),
		mariadb.Spec.Galera.Agent.Port,
	)
}
