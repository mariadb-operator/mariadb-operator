package galera

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GaleraReconciler) reconcileGaleraRecovery(ctx context.Context) error {
	log.FromContext(ctx).V(1).Info("Recovering Galera cluster")

	// TODO: perform galera recovery orchestrating requests to agents
	// See:
	// https://github.com/mariadb-operator/mariadb-ha-poc/blob/main/galera/kubernetes/2-crashrecovery.cnf
	// https://github.com/mariadb-operator/mariadb-ha-poc/blob/main/galera/kubernetes/1-bootstrap.cnf

	return nil
}
