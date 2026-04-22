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
	readOnlyLogger := logger.WithName("readonly")

	if !shouldReconcileReadOnly(mariadb, readOnlyLogger) {
		return ctrl.Result{}, nil
	}
	readOnlyDesiredPodState := r.getReadOnlyDesiredPodState(mariadb)

	clientSet := sql.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

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

func shouldReconcileReadOnly(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) bool {
	// Cordoned is a special not Ready condition, where we should allow setting readonly during maintenance, which includes cordoning.
	if mdb.IsCordoned() {
		return true
	}
	// Reconciling readonly when MariaDB is not ready has multiple negative effects. For example:
	// If the readonly reconciliation happens while MaxScale is performing a failover, the failover could hang with the following MaxScale logs:
	//
	// [mariadbmon] Failover 'mariadb-eu-central-1' -> 'mariadb-eu-central-0' performed.
	// notice : Server changed state: mariadb-eu-central-0[mariadb-eu-central-0.mariadb-eu-central-internal.default.svc.cluster.local:3306]: new_master. [Slave, Running] -> [Master, Running]
	// warning: [mariadbmon] The current primary server 'mariadb-eu-central-0' is no longer valid because it is in read-only mode, but there is no valid alternative to swap to.
	// notice : Server changed state: mariadb-eu-central-0[mariadb-eu-central-0.mariadb-eu-central-internal.default.svc.cluster.local:3306]: new_slave. [Master, Running] -> [Slave, Running]
	// error  : [readwritesplit] (rw-router); Couldn't find suitable Primary from 2 candidates.
	// error  : (rw-router); Failed to create new router session for service 'rw-router'. See previous errors for more details.
	//
	// Preventing this readonly reconciliation from triggering using this guard, the failover succeeds:
	//
	// notice : [mariadbmon] Failover 'mariadb-eu-central-0' -> 'mariadb-eu-central-1' performed.
	// notice : Server changed state: mariadb-eu-central-1[mariadb-eu-central-1.mariadb-eu-central-internal.default.svc.cluster.local:3306]: new_master. [Slave, Running] -> [Master, Running]
	// notice : Server changed state: mariadb-eu-central-0[mariadb-eu-central-0.mariadb-eu-central-internal.default.svc.cluster.local:3306]: server_up. [Down] -> [Running]
	// notice : [mariadbmon] Server 'mariadb-eu-central-0' is replicating from a server other than 'mariadb-eu-central-1', redirecting it to 'mariadb-eu-central-1'.
	// notice : [mariadbmon] 1 server(s) redirected or rejoined the cluster.
	// notice : Server changed state: mariadb-eu-central-0[mariadb-eu-central-0.mariadb-eu-central-internal.default.svc.cluster.local:3306]: new_slave. [Running] -> [Slave, Running]
	//
	// It is important to note that this issue only happens with the MaxScale failover, as it runs in parallel with the MariaDB controller i.e. race condition.
	// MariaDB failover is not affected, as it is handled by a previous stage in the same MariaDB controller.
	if !mdb.IsReady() {
		logger.V(1).Info("MariaDB is not ready. Skipping readonly reconciliation...")
		return false
	}
	return true
}
