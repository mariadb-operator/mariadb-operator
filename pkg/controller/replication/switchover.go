package replication

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controllers/resources"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type switchoverPhase struct {
	name      string
	reconcile func(context.Context, *mariadbv1alpha1.MariaDB, *replicationClientSet, logr.Logger) error
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *reconcileRequest, switchoverLogger logr.Logger) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil
	}
	if *req.mariadb.Replication().Primary.PodIndex == *req.mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}

	fromIndex := *req.mariadb.Status.CurrentPrimaryPodIndex
	toIndex := *req.mariadb.Replication().Primary.PodIndex
	logger := switchoverLogger.WithValues("mariadb", req.mariadb.Name, "from-index", fromIndex, "to-index", toIndex)
	logger.Info("Switching primary")
	r.recorder.Eventf(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimarySwitching,
		"Switching primary from index '%d' to index '%d'", fromIndex, toIndex)

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		conditions.SetPrimarySwitching(&req.mariadb.Status, req.mariadb)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}

	phases := []switchoverPhase{
		{
			name:      "Set read_only in current primary",
			reconcile: r.currentPrimaryReadOnly,
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
		if err := p.reconcile(ctx, req.mariadb, req.clientSet, logger); err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.UpdateCurrentPrimary(req.mariadb, toIndex)
		conditions.SetPrimarySwitched(&req.mariadb.Status)
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.Info("Primary switched")
	r.recorder.Eventf(req.mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimarySwitched,
		"Primary switched from index '%d' to index '%d'", fromIndex, toIndex)
	return nil
}

func (r *ReplicationReconciler) currentPrimaryReadOnly(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *replicationClientSet, logger logr.Logger) error {
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

	logger.Info("Enabling readonly mode in current primary")
	r.recorder.Event(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryReadonly,
		"Enabling readonly mode in current primary")
	return client.EnableReadOnly(ctx)
}

func (r *ReplicationReconciler) waitForReplicaSync(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *replicationClientSet, logger logr.Logger) error {
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
	primaryGtid, err := client.SystemVariable(ctx, "gtid_binlog_pos")
	if err != nil {
		return fmt.Errorf("error getting primary GTID binlog pos: %v", err)
	}

	var wg sync.WaitGroup
	doneChan := make(chan struct{})
	errChan := make(chan error)

	logger.Info("Waiting for replicas to be synced with primary")
	r.recorder.Event(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationReplicaSync,
		"Waiting for replicas to be synced with primary")
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
			timeout := mariadb.Replication().Replica.SyncTimeout.Duration
			if err := replClient.WaitForReplicaGtid(ctx, primaryGtid, timeout); err != nil {
				var errBundle *multierror.Error
				errBundle = multierror.Append(errBundle, fmt.Errorf("error waiting for GTID '%s' in replica '%d': %v", primaryGtid, i, err))

				if errors.Is(err, mariadbclient.ErrWaitReplicaTimeout) {
					logger.Error(err, "Timeout waiting for GTID in replica", "gtid", primaryGtid, "replica", i, "timeout", timeout)
					r.recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationReplicaSyncErr,
						"Timeout(%s) waiting for GTID '%s' in replica '%d': %v", timeout, primaryGtid, i, err)

					if err := r.resetSlave(ctx, replClient); err != nil {
						logger.Error(err, "Error resetting slave in replica after GTID timeout", "replica", i)
						errBundle = multierror.Append(errBundle, fmt.Errorf("error resetting slave position in replica '%d': %v", i, err))
					}
				} else {
					logger.Error(err, "Error waiting for GTID in replica", "gtid", primaryGtid, "replica", i)
					r.recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonReplicationReplicaSyncErr,
						"Error waiting for GTID '%s' in replica '%d': %v", primaryGtid, i, err)
				}

				errChan <- errBundle.ErrorOrNil()
				return
			}

			logger.V(1).Info("Replica synced, resetting slave position", "replica", i, "gtid", primaryGtid)
			if err := r.resetSlave(ctx, replClient); err != nil {
				logger.Error(err, "Error resetting slave in replica after synced", "replica", i)
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
	clientSet *replicationClientSet, logger logr.Logger) error {
	client, err := clientSet.newPrimaryClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	podIndex := *mariadb.Replication().Primary.PodIndex
	logger.Info("Configuring new primary", "pod-index", podIndex)
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryNew,
		"Configuring new primary at index '%d'", podIndex)

	if err := r.replConfig.ConfigurePrimary(ctx, mariadb, client, podIndex); err != nil {
		return fmt.Errorf("error confguring new primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) connectReplicasToNewPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *replicationClientSet, logger logr.Logger) error {
	var wg sync.WaitGroup
	doneChan := make(chan struct{})
	errChan := make(chan error)

	logger.Info("Connecting replicas to new primary")
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationReplicaConn, "Connecting replicas to new primary")

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex || i == *mariadb.Replication().Primary.PodIndex {
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
				logger.V(1).Info("Error getting Pod when connecting replicas to new primary", "pod", key.Name)
				if apierrors.IsNotFound(err) {
					return
				}
				errChan <- err
				return
			}
			if !mariadbpod.PodReady(&pod) {
				logger.V(1).Info("Skipping non ready Pod when connecting replicas to new primary", "pod", key.Name)
				return
			}

			replClient, err := clientSet.clientForIndex(ctx, i)
			if err != nil {
				errChan <- fmt.Errorf("error getting replica '%d' client: %v", i, err)
				return
			}

			logger.V(1).Info("Connecting replica to new primary", "replica", i)
			if err := r.replConfig.ConfigureReplica(ctx, mariadb, replClient, i, *mariadb.Replication().Primary.PodIndex); err != nil {
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
	clientSet *replicationClientSet, logger logr.Logger) error {
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

	currentPrimary := *mariadb.Status.CurrentPrimaryPodIndex
	newPrimary := *mariadb.Replication().Primary.PodIndex
	logger.Info("Change current primary to be a replica", "current-primary", currentPrimary, "new-primary", newPrimary)
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimaryToReplica,
		"Change current primary '%d' to be a replica. New primary at '%d'", currentPrimary, newPrimary)

	return r.replConfig.ConfigureReplica(
		ctx,
		mariadb,
		currentPrimaryClient,
		currentPrimary,
		newPrimary,
	)
}

func (r *ReplicationReconciler) updatePrimaryService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *replicationClientSet, logger logr.Logger) error {
	key := ctrlresources.PrimaryServiceKey(mariadb)
	var service corev1.Service
	if err := r.Get(ctx, key, &service); err != nil {
		return fmt.Errorf("error getting Service: %v", err)
	}

	podIndex := *mariadb.Replication().Primary.PodIndex
	logger.Info("Update primary service", "pod-index", podIndex)
	r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationPrimarySvcUpdate,
		"Update primary service pointing to index '%d'", podIndex)

	serviceLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			WithStatefulSetPod(mariadb, podIndex).
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
