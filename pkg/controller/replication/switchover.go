package replication

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	replresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/replication/resources"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		return conditions.SetPrimarySwitching(&req.mariadb.Status, req.mariadb)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger := log.FromContext(
		ctx,
		"mariadb",
		req.mariadb.Name,
		"fromIndex",
		*req.mariadb.Status.CurrentPrimaryPodIndex,
		"toIndex",
		req.mariadb.Spec.Replication.Primary.PodIndex,
	)
	logger.Info("Switching primary")

	phases := []switchoverPhase{
		{
			name:      "Lock current primary tables",
			reconcile: r.lockCurrentPrimary,
		},
		{
			name:      "Wait for replica sync",
			reconcile: r.waitForReplicaSync,
		},
		{
			name:      "Configure new primary",
			reconcile: r.configureNewPrimary,
		},
		{
			name:      "Connect replicas to new primary",
			reconcile: r.connectReplicasToNewPrimary,
		},
		{
			name:      "Change current primary to replica",
			reconcile: r.changeCurrentPrimaryToReplica,
		},
		{
			name:      "Upgrade primary Service",
			reconcile: r.updatePrimaryService,
		},
	}

	for _, p := range phases {
		logger.Info(p.name)
		if err := p.reconcile(ctx, req.mariadb, req.clientSet); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.UpdateCurrentPrimary(req.mariadb, req.mariadb.Spec.Replication.Primary.PodIndex)
		conditions.SetPrimarySwitched(&req.mariadb.Status)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.Info("Switched primary")

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

	return client.LockTablesWithReadLock(ctx)
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

			logger.V(1).Info("Syncing replica with primary GTID", "replica", i, "gtid", primaryGtid)
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

				errChan <- errBundle.ErrorOrNil()
				return
			}

			logger.V(1).Info("Replica synced, resetting slave position", "replica", i, "gtid", primaryGtid)
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

	if err := r.ReplConfig.ConfigurePrimary(ctx, mariadb, client, mariadb.Spec.Replication.Primary.PodIndex); err != nil {
		return fmt.Errorf("error confguring new primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) connectReplicasToNewPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	logger := log.FromContext(ctx)
	var wg sync.WaitGroup
	doneChan := make(chan struct{})
	errChan := make(chan error)

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex || i == mariadb.Spec.Replication.Primary.PodIndex {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := types.NamespacedName{
				Name:      statefulset.PodName(mariadb.ObjectMeta, i),
				Namespace: mariadb.Namespace,
			}
			var pod corev1.Pod
			if err := r.Get(ctx, key, &pod); err != nil {
				if apierrors.IsNotFound(err) {
					return
				}
				errChan <- err
				return
			}
			if !mariadbpod.PodReady(&pod) {
				return
			}

			replClient, err := clientSet.clientForIndex(ctx, i)
			if err != nil {
				errChan <- fmt.Errorf("error getting replica '%d' client: %v", i, err)
				return
			}

			logger.V(1).Info("Connecting replica to new primary", "replica", i)
			if err := r.ReplConfig.ConfigureReplica(ctx, mariadb, replClient, i, mariadb.Spec.Replication.Primary.PodIndex); err != nil {
				errChan <- fmt.Errorf("error configuring replica vars in replica '%d': %v", i, err)
				return
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
	return r.ReplConfig.ConfigureReplica(
		ctx,
		mariadb,
		currentPrimaryClient,
		*mariadb.Status.CurrentPrimaryPodIndex,
		mariadb.Spec.Replication.Primary.PodIndex,
	)
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

	return r.Patch(ctx, &service, patch)
}

func (r *ReplicationReconciler) resetSlave(ctx context.Context, client *mariadbclient.Client) error {
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := client.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	return client.StartSlave(ctx, connectionName)
}

func (r *ReplicationReconciler) currentPrimaryReady(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	podName := statefulset.PodName(mariadb.ObjectMeta, *mariadb.Status.CurrentPrimaryPodIndex)
	key := types.NamespacedName{
		Name:      podName,
		Namespace: mariadb.Namespace,
	}
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return false, err
	}
	return mariadbpod.PodReady(&pod), nil
}
