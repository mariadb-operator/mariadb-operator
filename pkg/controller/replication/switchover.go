package replication

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	replresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/replication/resources"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type switchoverPhase struct {
	name      string
	reconcile func(context.Context, *mariadbv1alpha1.MariaDB, *mariadbClientSet) error
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil
	}
	if req.mariadb.Spec.Replication.Primary.PodIndex == *req.mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		var errBundle *multierror.Error
		err := conditions.SetReadySwitchingPrimary(&req.mariadb.Status, req.mariadb)
		errBundle = multierror.Append(errBundle, err)

		err = conditions.SetPrimarySwitchedInProgress(&req.mariadb.Status, req.mariadb)
		errBundle = multierror.Append(errBundle, err)
		return errBundle.ErrorOrNil()
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger := log.FromContext(ctx)
	logger.Info(
		"switching primary",
		"fromIndex",
		*req.mariadb.Status.CurrentPrimaryPodIndex,
		"toIndex",
		req.mariadb.Spec.Replication.Primary.PodIndex,
	)

	phases := []switchoverPhase{
		{
			name:      "lock current primary tables",
			reconcile: r.lockCurrentPrimary,
		},
		{
			name:      "wait for replica sync",
			reconcile: r.waitForReplicaSync,
		},
		{
			name:      "configure new primary",
			reconcile: r.configureNewPrimary,
		},
		{
			name:      "connect replicas to new primary",
			reconcile: r.connectReplicasToNewPrimary,
		},
		{
			name:      "change current primary to replica",
			reconcile: r.changeCurrentPrimaryToReplica,
		},
		{
			name:      "upgrade primary Service",
			reconcile: r.updatePrimaryService,
		},
	}

	for _, p := range phases {
		logger.Info(p.name)
		if err := p.reconcile(ctx, req.mariadb, req.clientSet); err != nil {
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.UpdateCurrentPrimaryStatus(req.mariadb, req.mariadb.Spec.Replication.Primary.PodIndex)
		conditions.SetPrimarySwitchedComplete(&req.mariadb.Status)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.Info("primary switchover completed")

	return nil
}

func (r *ReplicationReconciler) lockCurrentPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	ready, err := r.currentPrimaryReady(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return nil
	}
	client, err := clientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	if err := client.LockTablesWithReadLock(ctx); err != nil {
		return fmt.Errorf("error locking tables in primary: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) waitForReplicaSync(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	ready, err := r.currentPrimaryReady(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return nil
	}
	client, err := clientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}
	primaryGtid, err := client.GlobalVar(ctx, "gtid_binlog_pos")
	if err != nil {
		return fmt.Errorf("error getting primary GTID binlog pos: %v", err)
	}

	logger := log.FromContext(ctx)
	var wg sync.WaitGroup
	doneChan := make(chan struct{})
	errChan := make(chan error)

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			replClient, err := clientSet.clientForIndex(ctx, i)
			if err != nil {
				errChan <- fmt.Errorf("error getting replica '%d' client: %v", i, err)
				return
			}

			logger.V(1).Info("syncing replica with primary GTID", "replica", i, "gtid", primaryGtid)
			if err := replClient.WaitForReplicaGtid(
				ctx,
				primaryGtid,
				mariadb.Spec.Replication.Replica.SyncTimeoutOrDefault(),
			); err != nil {
				var errBundle *multierror.Error
				errBundle = multierror.Append(errBundle, fmt.Errorf("error waiting for GTID '%s' in replica '%d'", err, i))

				if errors.Is(err, mariadbclient.ErrWaitReplicaTimeout) {
					if err := r.resetSlave(ctx, replClient); err != nil {
						errBundle = multierror.Append(errBundle, fmt.Errorf("error resetting slave position in replica '%d': %v", i, err))
					}
				}

				errChan <- errBundle
				return
			}

			logger.V(1).Info("replica synced, resetting slave position", "replica", i, "gtid", primaryGtid)
			if err := r.resetSlave(ctx, replClient); err != nil {
				errChan <- fmt.Errorf("error resetting slave position in replica '%d' after being synced: %v", i, err)
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		return nil
	case err := <-errChan:
		return err
	}
}

func (r *ReplicationReconciler) configureNewPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	client, err := clientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	config := NewReplicationConfig(mariadb, client, r.Client)
	if err := config.ConfigurePrimary(ctx, mariadb.Spec.Replication.Primary.PodIndex); err != nil {
		return fmt.Errorf("error confguring new primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) connectReplicasToNewPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex || i == mariadb.Spec.Replication.Primary.PodIndex {
			continue
		}
		replClient, err := clientSet.clientForIndex(ctx, i)
		if err != nil {
			return fmt.Errorf("error getting replica '%d' client: %v", i, err)
		}

		config := NewReplicationConfig(mariadb, replClient, r.Client)
		if err := config.ConfigureReplica(ctx, i, mariadb.Spec.Replication.Primary.PodIndex); err != nil {
			return fmt.Errorf("error configuring replica vars in replica '%d': %v", err, i)
		}
	}
	return nil
}

func (r *ReplicationReconciler) changeCurrentPrimaryToReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	ready, err := r.currentPrimaryReady(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting current primary readiness: %v", err)
	}
	if !ready {
		return nil
	}
	currentPrimaryClient, err := clientSet.currentPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}

	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
		return fmt.Errorf("error getting replication password Secret: %v", err)
	}
	config := NewReplicationConfig(mariadb, currentPrimaryClient, r.Client)
	if err := config.ConfigureReplica(ctx, *mariadb.Status.CurrentPrimaryPodIndex, mariadb.Spec.Replication.Primary.PodIndex); err != nil {
		return fmt.Errorf("error configuring replica vars in current primary: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) updatePrimaryService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	key := replresources.PrimaryServiceKey(mariadb)
	var service corev1.Service
	if err := r.Get(ctx, key, &service); err != nil {
		return fmt.Errorf("error getting Service: %v", err)
	}

	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			WithStatefulSetPod(mariadb, mariadb.Spec.Replication.Primary.PodIndex).
			Build()
	patch := client.MergeFrom(service.DeepCopy())
	service.ObjectMeta.Labels = serviceLabels
	service.Spec.Selector = serviceLabels

	if err := r.Patch(ctx, &service, patch); err != nil {
		return fmt.Errorf("error patching Service: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) resetSlave(ctx context.Context, client *mariadbclient.Client) error {
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := client.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := client.StartSlave(ctx, connectionName); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) currentPrimaryReady(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	podName := statefulset.PodName(mariadb.ObjectMeta, *mariadb.Status.CurrentPrimaryPodIndex)
	key := types.NamespacedName{
		Name:      podName,
		Namespace: mariadb.Namespace,
	}
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return false, fmt.Errorf("error getting current primary Pod: %v", err)
	}
	return mariadbpod.PodReady(&pod), nil
}
