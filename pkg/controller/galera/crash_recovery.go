package galera

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GaleraReconciler) reconcileCrashRecovery(ctx context.Context) error {
	log.FromContext(ctx).V(1).Info("Recovering Galera cluster")
	return nil
}
