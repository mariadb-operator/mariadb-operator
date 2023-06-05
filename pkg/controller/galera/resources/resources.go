package replication

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	GaleraCnf          = "0-galera.cnf"
	GaleraBootstrapCnf = "1-bootstrap.cnf"
	GaleraInitScript   = "init.sh"

	GaleraConfigMapVolume    = "galera-configmap"
	GaleraConfigMapMountPath = "/galera"
	GaleraConfigVolume       = "galera"
	GaleraConfigMountPath    = "/etc/mysql/mariadb.conf.d"

	GaleraClusterPortName = "cluster"
	GaleraClusterPort     = int32(4444)
	GaleraISTPortName     = "ist"
	GaleraISTPort         = int32(4567)
	GaleraSSTPortName     = "sst"
	GaleraSSTPort         = int32(4568)
	AgentPortName         = "agent"
)

func ConfigMapKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("config-galera-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func ServiceKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-internal", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
