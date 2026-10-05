package replication

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileSemiSyncWhenReplicasAvailable runs 'reconcileSemiSync' as soon as the primary has at least one
// replica registered as a semi-synchronous client, and therefore able to acknowledge.
//
// 'reconcileSemiSync' only runs after every Pod has been configured, so a non-ready Pod keeps the primary unarmed for as
// long as it is unreachable. For instance, a primary that goes down without a switchover, either hard deleted or drained
// while MaxScale is unavailable, and stays Pending afterwards: MaxScale fails over to a replica, which stays unarmed until the
// old primary comes back. Arming it here is safe, since a registered client is able to acknowledge the replication user DDL
// in 'ConfigurePrimary'.
//
// When the primary is unreachable it defers to the Pod loop and to 'reconcileSemiSync' afterwards, without requeuing, so it
// never blocks reconciliation.
func (r *ReplicationReconciler) reconcileSemiSyncWhenReplicasAvailable(ctx context.Context, req *ReconcileRequest,
	logger logr.Logger) error {
	if !req.mariadb.IsSemiSyncEnabled() {
		return nil
	}
	podLogger := logger.WithValues("pod", statefulset.PodName(req.mariadb.ObjectMeta, *req.currentPrimaryPodIndex))

	client, err := req.replClientSet.clientForIndex(ctx, *req.currentPrimaryPodIndex)
	if err != nil {
		podLogger.V(1).Info("Unable to get primary client to get semi-sync clients", "err", err)
		return nil
	}
	clients, err := client.SemiSyncMasterClients(ctx)
	if err != nil {
		podLogger.V(1).Info("Unable to get semi-sync clients", "err", err)
		return nil
	}
	if clients < 1 {
		podLogger.V(1).Info("No semi-sync clients registered in primary. Skipping semi-sync reconciliation...")
		return nil
	}

	if err := r.reconcileSemiSync(ctx, req, *req.currentPrimaryPodIndex, logger); err != nil {
		return fmt.Errorf("error reconciling semi-sync: %w", err)
	}
	return nil
}

// ReconcileSemiSyncWhenReplicasAvailable runs 'reconcileSemiSyncWhenReplicasAvailable' outside of the Replication phase,
// resolving the current primary the same way. The rolling update uses it, since it returns before reaching that phase.
func (r *ReplicationReconciler) ReconcileSemiSyncWhenReplicasAvailable(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	if !mdb.IsSemiSyncEnabled() {
		return nil
	}
	logger := log.FromContext(ctx).WithName("replication")

	primaryPodIndex, err := r.currentPrimaryPodIndex(ctx, mdb, logger)
	if err != nil {
		return fmt.Errorf("error getting current primary Pod index: %w", err)
	}
	if primaryPodIndex == nil {
		logger.V(1).Info("Primary Pod index not found. Skipping semi-sync reconciliation...")
		return nil
	}

	req, err := r.NewReconcileRequest(ctx, mdb, primaryPodIndex)
	if err != nil {
		return fmt.Errorf("error creating reconcile request: %v", err)
	}
	defer req.Close()

	return r.reconcileSemiSyncWhenReplicasAvailable(ctx, req, logger.WithName("semi-sync"))
}

// reconcileSemiSync converges rpl_semi_sync_master_enabled to the role of each Pod: enabled in the primary and disabled in
// the replicas. rpl_semi_sync_slave_enabled stays enabled everywhere, it is rendered in the configuration file.
//
// Primary-side semi-synchronous replication used to be enabled in every node, so that any of them could be promoted without
// reconfiguration. As a side effect, a node with no replicas connected to it was still a semi-synchronous primary: with
// rpl_semi_sync_master_wait_no_slave=ON, every statement it writes to its own binary log waits for an acknowledgement that
// can never arrive, for the whole semiSyncAckTimeout. With a high timeout, it never recovers. This affects the replication
// user reconciliation in ConfigureReplica, any binlogged statement in a replica, a primary being provisioned or promoted
// before the replicas are pointed at it, and the transactions a replica applies when log_slave_updates=ON.
//
// This is level-triggered on purpose, and it is the only place that sets the variable: with semiSyncBootAsReplica enabled the
// configuration file renders rpl_semi_sync_master_enabled=OFF, so a Pod always boots unarmed and this function converges it,
// on every reconcile and after every restart. Nothing needs to arm a Pod at its role transition, which is why
// 'ConfigurePrimary' and 'ConfigureReplica' do not, and it means a variable flipped by hand or by a restart is corrected on
// the next reconcile.
//
// It must run after the Pods have been configured, so that the primary is only armed once the replicas have been pointed at
// it and can acknowledge. Arming it earlier would make the replication user DDL in 'ConfigurePrimary' wait for the whole
// semiSyncAckTimeout. The node is not writable in the meantime, see 'NewReplicationConfig'.
func (r *ReplicationReconciler) reconcileSemiSync(ctx context.Context, req *ReconcileRequest, primaryPodIndex int,
	logger logr.Logger) error {
	if !req.mariadb.IsSemiSyncEnabled() {
		return nil
	}

	for i := 0; i < int(req.mariadb.Spec.Replicas); i++ {
		podLogger := logger.WithValues("pod", statefulset.PodName(req.mariadb.ObjectMeta, i))

		// Not being able to connect to a Pod is never fatal, not even for the primary: it is expected while the cluster
		// converges, and also while the root password is being rotated. The operator authenticates with
		// 'spec.rootPasswordSecretKeyRef', which already holds the new password while the servers still have the old one
		// until the 'Root Password' phase, which runs after this one. Returning an error here would abort the phase loop
		// before that phase gets a chance to rotate the password, so the rotation would never complete and the connection
		// would never succeed. The next reconciliation retries.
		client, err := req.replClientSet.clientForIndex(ctx, i)
		if err != nil {
			podLogger.V(1).Info("Unable to get client to reconcile semi-sync", "err", err)
			continue
		}

		if err := r.reconcileSemiSyncInPod(ctx, req, i, i == primaryPodIndex, client, podLogger); err != nil {
			if i == primaryPodIndex {
				return fmt.Errorf("error reconciling semi-sync in primary Pod %d: %w", i, err)
			}
			// An unreachable Pod is expected while the cluster converges. The next reconciliation retries.
			podLogger.V(1).Info("Unable to reconcile semi-sync", "err", err)
		}
	}
	return nil
}

func (r *ReplicationReconciler) reconcileSemiSyncInPod(ctx context.Context, req *ReconcileRequest, podIndex int,
	shouldBeEnabled bool, client *sqlClient.Client, logger logr.Logger) error {
	isEnabled, err := client.IsSemiSyncMasterEnabled(ctx)
	if err != nil {
		return fmt.Errorf("error getting semi-sync master enabled: %v", err)
	}
	// Nothing to converge: the Events below are only emitted on an actual role transition, and not on every reconcile.
	if isEnabled == shouldBeEnabled {
		return nil
	}
	pod := statefulset.PodName(req.mariadb.ObjectMeta, podIndex)

	if shouldBeEnabled {
		if err := client.EnableSemiSyncMaster(ctx); err != nil {
			return fmt.Errorf("error enabling semi-sync master: %v", err)
		}
		logger.Info("Enabled primary-side semi-synchronous replication")
		r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationSemiSyncEnabled,
			mariadbv1alpha1.ReasonReplicationSemiSyncEnabled, "Enabled primary-side semi-synchronous replication in Pod '%s'", pod)
		return nil
	}
	if err := client.DisableSemiSyncMaster(ctx); err != nil {
		return fmt.Errorf("error disabling semi-sync master: %v", err)
	}
	logger.Info("Disabled primary-side semi-synchronous replication")
	r.recorder.Eventf(req.mariadb, nil, corev1.EventTypeNormal, mariadbv1alpha1.ReasonReplicationSemiSyncDisabled,
		mariadbv1alpha1.ReasonReplicationSemiSyncDisabled, "Disabled primary-side semi-synchronous replication in Pod '%s'", pod)
	return nil
}

// reconcileSemiSyncSwitchover is the switchover phase that arms the new primary and disarms the demoted one. It runs after
// the replicas have been connected to the new primary, and it must not use 'status.currentPrimaryPodIndex', which still
// points to the old primary until the switchover completes.
func (r *ReplicationReconciler) reconcileSemiSyncSwitchover(ctx context.Context, req *ReconcileRequest,
	logger logr.Logger) error {
	replication := ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	return r.reconcileSemiSync(ctx, req, *replication.Primary.PodIndex, logger)
}

// disableNewPrimaryReadOnly hands write traffic to the new primary, and runs after 'reconcileSemiSyncSwitchover' has armed it.
// In steady state this is the Maintenance phase's job, but a switchover must leave a writable primary behind when it completes,
// rather than one that stays read_only until a later phase in a later reconciliation gets to it.
func (r *ReplicationReconciler) disableNewPrimaryReadOnly(ctx context.Context, req *ReconcileRequest,
	logger logr.Logger) error {
	replication := ptr.Deref(req.mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	podIndex := *replication.Primary.PodIndex

	client, err := req.replClientSet.clientForIndex(ctx, podIndex)
	if err != nil {
		return fmt.Errorf("error getting client to disable read_only in Pod %d: %v", podIndex, err)
	}
	if err := client.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling read_only in Pod %d: %v", podIndex, err)
	}
	logger.Info("Disabled read_only in new primary", "pod", statefulset.PodName(req.mariadb.ObjectMeta, podIndex))
	return nil
}
