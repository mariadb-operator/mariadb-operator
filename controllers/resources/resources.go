package resources

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func InternalServiceKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-internal", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func PrimaryServiceKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("primary-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func PrimaryConnectioneKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("primary-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}
