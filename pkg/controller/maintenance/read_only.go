package maintenance

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
)

func (r *MaintenanceReconciler) reconcileReadOnly(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) error {
	readOnlyDesiredPodState := r.getReadOnlyDesiredPodState(mariadb)

	clientSet := sql.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

	readOnlyLogger := logger.WithName("readonly")
	readOnlyLogger.V(1).Info("Reconciling readonly", "desired-state", readOnlyDesiredPodState)

	for podIndex, desiredReadOnly := range readOnlyDesiredPodState {
		podName := statefulset.PodName(mariadb.ObjectMeta, podIndex)
		podLogger := readOnlyLogger.WithValues("pod", podName)

		client, err := clientSet.ClientForIndex(ctx, podIndex)
		if err != nil {
			// We shouldn't be returning an error here, as it would hang the reconciliation i.e. next phases won't be executed.
			// Readonly will be reconciled once a SQL connection can be established.
			podLogger.V(1).Info("Error getting SQL client for Pod index", "err", err)
			return nil
		}
		currentReadOnly, err := client.GetReadOnly(ctx)
		if err != nil {
			return fmt.Errorf("error getting readonly state in Pod %s: %v", podName, err)
		}

		if desiredReadOnly == currentReadOnly {
			podLogger.V(1).Info("Desired state matches current state, nothing to do")
			continue
		}
		if desiredReadOnly {
			podLogger.Info("Enabling readonly")
			r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMaintenance, mariadbv1alpha1.ActionReconciling,
				"Enabling readonly in Pod %s", podName)

			if err := client.EnableReadOnly(ctx); err != nil {
				return fmt.Errorf("error enabling readonly in Pod %s %v", podName, err)
			}
		} else {
			podLogger.Info("Disabling readonly")
			r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMaintenance, mariadbv1alpha1.ActionReconciling,
				"Disabling readonly in Pod %s", podName)

			if err := client.DisableReadOnly(ctx); err != nil {
				return fmt.Errorf("error disabling readonly in Pod %s: %v", podName, err)
			}
		}
	}
	return nil
}

// getReadOnlyDesiredPodState returns a map with the desired readOnly state indexed by Pod index.
func (r *MaintenanceReconciler) getReadOnlyDesiredPodState(mariadb *mariadbv1alpha1.MariaDB) map[int]bool {
	desiredReadOnly := mariadb.IsReadOnlyEnabled()
	desiredReadOnlyPodState := make(map[int]bool, mariadb.Spec.Replicas)

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if mariadb.IsReplicationEnabled() {
			if mariadb.Status.CurrentPrimaryPodIndex != nil && *mariadb.Status.CurrentPrimaryPodIndex == i {
				desiredReadOnlyPodState[i] = desiredReadOnly
			} else {
				desiredReadOnlyPodState[i] = true
			}
		} else {
			desiredReadOnlyPodState[i] = desiredReadOnly
		}
	}
	return desiredReadOnlyPodState
}
