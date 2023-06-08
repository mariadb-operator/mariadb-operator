package galera

import (
	"github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

// nolint
type agentClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	clientByIndex map[int]client.Client
}
