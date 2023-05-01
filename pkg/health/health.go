package health

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func IsMariaDBHealthy(ctx context.Context, client ctrlclient.Client, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	key := ctrlclient.ObjectKeyFromObject(mariadb)
	var statefulSet appsv1.StatefulSet
	if err := client.Get(ctx, key, &statefulSet); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}
	if statefulSet.Status.ReadyReplicas != mariadb.Spec.Replicas {
		return false, nil
	}
	var endpoints corev1.Endpoints
	if err := client.Get(ctx, key, &endpoints); err != nil {
		return false, ctrlclient.IgnoreNotFound(err)
	}
	for _, subset := range endpoints.Subsets {
		for _, port := range subset.Ports {
			if port.Port == mariadb.Spec.Port {
				return len(subset.Addresses) == int(mariadb.Spec.Replicas), nil
			}
		}
	}
	return false, nil
}
