package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MaintenanceReconciler) reconcileReadOnly(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
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
			// This is to avoid noisy error logs, as it is continuously reconciling.
			podLogger.V(1).Info("Error getting SQL client for Pod index", "err", err)
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
		currentReadOnly, err := client.GetReadOnly(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting readonly state in Pod %s: %v", podName, err)
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
				return ctrl.Result{}, fmt.Errorf("error enabling readonly in Pod %s %v", podName, err)
			}
		} else {
			podLogger.Info("Disabling readonly")
			r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMaintenance, mariadbv1alpha1.ActionReconciling,
				"Disabling readonly in Pod %s", podName)

			if err := client.DisableReadOnly(ctx); err != nil {
				return ctrl.Result{}, fmt.Errorf("error disabling readonly in Pod %s: %v", podName, err)
			}
		}
	}
	return ctrl.Result{}, nil
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
