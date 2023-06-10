package galera

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GaleraReconciler) reconcileGaleraRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	log.FromContext(ctx).V(1).Info("Recovering Galera cluster")

	// TODO: perform galera recovery orchestrating requests to agents
	// See:
	// https://github.com/mariadb-operator/mariadb-ha-poc/blob/main/galera/kubernetes/2-crashrecovery.cnf
	// https://github.com/mariadb-operator/mariadb-ha-poc/blob/main/galera/kubernetes/1-bootstrap.cnf
	//	Maybe useful for recovery?
	//	- SHOW STATUS LIKE 'wsrep_local_state_comment';
	//		The output will display the state of each node, such as "Synced," "Donor," "Joining," "Joined," or "Disconnected."
	//		All nodes should ideally be in the "Synced" state.

	_, err := newAgentClientSet(mariadb)
	if err != nil {
		return fmt.Errorf("error getting agent client: %v", err)
	}

	// state, err := agentClient.GaleraState.Get(ctx)
	// if err != nil {
	// 	if agentclient.IsNotFound(err) {
	// 		return true, nil
	// 	}
	// 	return false, fmt.Errorf("error getting galera state: %v", err)
	// }
	// if !state.SafeToBootstrap {
	// 	return false, nil
	// }

	return nil
}
