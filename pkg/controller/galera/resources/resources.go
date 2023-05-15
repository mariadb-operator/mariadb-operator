package replication

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func ConfigMapKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("config-galera-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func PodDisruptionBudgetKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      mariadb.Name,
		Namespace: mariadb.Namespace,
	}
}
