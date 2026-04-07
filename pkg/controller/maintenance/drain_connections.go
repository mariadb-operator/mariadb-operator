package maintenance

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MaintenanceReconciler) reconcileDrainConnections(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	maintenance := ptr.Deref(mariadb.Spec.Maintenance, mariadbv1alpha1.MariaDBMaintenance{})
	if !maintenance.DrainConnections {
		return ctrl.Result{}, nil
	}
	drainLogger := logger.WithName("drain")

	g := new(errgroup.Group)
	g.SetLimit(int(mariadb.Spec.Replicas))

	clientSet := sql.NewClientSet(mariadb, r.refResolver)
	defer clientSet.Close()

	for i, result := range clientSet.Clients(ctx) {
		g.Go(func() error {
			podName := statefulset.PodName(mariadb.ObjectMeta, i)
			if result.Err != nil {
				return fmt.Errorf("could not create maintenance client for %s: %v", podName, result.Err)
			}
			client := result.Client

			processes, err := client.GetProcessList(ctx)
			if err != nil {
				return fmt.Errorf("error fetching processlist for %s: %v", podName, err)
			}

			return r.drainProcesses(ctx, client, processes, mariadb, drainLogger.WithValues("pod", podName))
		})
	}

	return ctrl.Result{}, g.Wait()
}

func (r *MaintenanceReconciler) drainProcesses(ctx context.Context, client *sql.Client, processes []sql.Process,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	for _, process := range processes {
		plogger := logger.WithValues("id", process.ID, "command", process.Command, "time", process.Time)
		plogger.V(1).Info("Evaluating candidate process for draining")

		if process.Time > mariadb.Spec.Maintenance.DrainGracePeriodSeconds && process.IsSafeToTerminate() {
			plogger.Info("Draining process")
			r.recorder.Eventf(mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonMaintenance, mariadbv1alpha1.ActionReconciling,
				"Draining process (id=%d,command=%s,time=%d)", process.ID, process.Command, process.Time)

			if err := client.SoftKillProcess(ctx, process); sql.IgnoreYouAreNotOwnerOfThread(err) != nil {
				return fmt.Errorf("error killing process ID(%d) Command(%s): %v", process.ID, process.Command, err)
			}
		}
	}
	return nil
}
