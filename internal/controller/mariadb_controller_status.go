package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/replication"
	mdbpod "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	stspkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mdb.IsSuspended() {
		return ctrl.Result{}, r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetReadySuspended(status)
			return nil
		})
	}
	logger := log.FromContext(ctx).WithName("status").V(1)

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mdb), &sts); err != nil {
		logger.Info("error getting StatefulSet", "err", err)
	}

	replState, replErr := r.getReplicationState(ctx, mdb)
	if replErr != nil {
		logger.Info("error getting replication state", "err", replErr)
	}
	replErrStatus, replErrStatusErr := r.getReplicationErrors(ctx, mdb)
	if replErrStatusErr != nil {
		logger.Info("error getting replication error status", "err", replErrStatus)
	}

	mxsPrimaryPodIndex, mxsErr := r.getMaxScalePrimaryPod(ctx, mdb)
	if mxsErr != nil {
		logger.Info("error getting MaxScale primary Pod", "err", mxsErr)
	}

	tlsStatus, err := r.getTLSStatus(ctx, mdb)
	if err != nil {
		logger.Info("error getting TLS status", "err", err)
	}

	return ctrl.Result{}, r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.DefaultVersion = r.Environment.MariadbDefaultVersion
		status.Replicas = sts.Status.ReadyReplicas
		defaultPrimary(mdb)
		setMaxScalePrimary(mdb, mxsPrimaryPodIndex)

		if replState != nil {
			if status.Replication == nil {
				status.Replication = &mariadbv1alpha1.ReplicationStatus{}
			}
			status.Replication.State = replState
		}
		if replErrStatus != nil {
			if status.Replication == nil {
				status.Replication = &mariadbv1alpha1.ReplicationStatus{}
			}
			status.Replication.Errors = replErrStatus
		}

		if tlsStatus != nil {
			status.TLS = tlsStatus
		}

		if apierrors.IsNotFound(mxsErr) && !ptr.Deref(mdb.Spec.MaxScale, mariadbv1alpha1.MariaDBMaxScaleSpec{}).Enabled {
			r.ConditionReady.PatcherRefResolver(mxsErr, mariadbv1alpha1.MaxScale{})(&mdb.Status)
			return nil
		}
		if mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsSwitchingPrimary() || mdb.HasGaleraNotReadyCondition() {
			return nil
		}

		if err := r.setUpdatedCondition(ctx, mdb); err != nil {
			log.FromContext(ctx).V(1).Info("error setting MariaDB updated condition", "err", err)
		}
		condition.SetReadyWithMariaDB(&mdb.Status, &sts, mdb)
		return nil
	})
}

func (r *MariaDBReconciler) getReplicationState(ctx context.Context,
	mdb *mariadbv1alpha1.MariaDB) (map[string]mariadbv1alpha1.ReplicationState, error) {
	if !mdb.IsReplicationEnabled() {
		return nil, nil
	}

	clientSet, err := replication.NewReplicationClientSet(mdb, r.RefResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.Close()

	var replState map[string]mariadbv1alpha1.ReplicationState
	logger := log.FromContext(ctx)
	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		pod := stspkg.PodName(mdb.ObjectMeta, i)

		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			logger.V(1).Info("error getting client for Pod", "err", err, "pod", pod)
			continue
		}

		var aggErr *multierror.Error

		isReplica, err := client.IsReplicationReplica(ctx)
		aggErr = multierror.Append(aggErr, err)
		hasConnectedReplicas, err := client.HasConnectedReplicas(ctx)
		aggErr = multierror.Append(aggErr, err)

		if err := aggErr.ErrorOrNil(); err != nil {
			logger.V(1).Info("error checking Pod replication state", "err", err, "pod", pod)
			continue
		}

		state := mariadbv1alpha1.ReplicationStateNotConfigured
		if mdb.ReplicaNeedsConfiguration(pod) {
			state = mariadbv1alpha1.ReplicationStateConfiguring
		} else if isReplica {
			state = mariadbv1alpha1.ReplicationStateReplica
		} else if hasConnectedReplicas {
			state = mariadbv1alpha1.ReplicationStatePrimary
		}
		if replState == nil {
			replState = make(map[string]mariadbv1alpha1.ReplicationState)
		}
		replState[pod] = state
	}
	return replState, nil
}

func (r *MariaDBReconciler) getReplicationErrors(ctx context.Context,
	mdb *mariadbv1alpha1.MariaDB) (map[string]mariadbv1alpha1.ReplicaErrorStatus, error) {
	if !mdb.IsReplicationEnabled() {
		return nil, nil
	}
	replStatus := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})

	if mdb.Status.CurrentPrimaryPodIndex == nil {
		return replStatus.Errors, nil
	}

	clientSet, err := replication.NewReplicationClientSet(mdb, r.RefResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.Close()

	var replicaErrorStatus map[string]mariadbv1alpha1.ReplicaErrorStatus
	logger := log.FromContext(ctx)
	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		if i == *mdb.Status.CurrentPrimaryPodIndex {
			continue
		}
		pod := stspkg.PodName(mdb.ObjectMeta, i)

		var currentReplicaErrors *mariadbv1alpha1.ReplicaErrorStatus
		if current, ok := replStatus.Errors[pod]; ok {
			currentReplicaErrors = &current
		}
		// when the Pods are restarted or unstable, SQL connections could fail, keep the current state
		preserveCurrentState := func() {
			if replicaErrorStatus == nil {
				replicaErrorStatus = make(map[string]mariadbv1alpha1.ReplicaErrorStatus)
			}
			if currentReplicaErrors != nil {
				replicaErrorStatus[pod] = *currentReplicaErrors
			}
		}

		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			logger.V(1).Info("error getting client for Pod", "err", err, "pod", pod)
			preserveCurrentState()
			continue
		}

		replicaErrors, err := client.ReplicaErrors(ctx)
		if err != nil {
			logger.V(1).Info("error checking Pod replica errors", "err", err, "pod", pod)
			preserveCurrentState()
			continue
		}

		mergedReplicaErrors := mergeReplicaErrors(currentReplicaErrors, replicaErrors)
		if mergedReplicaErrors != nil {
			if replicaErrorStatus == nil {
				replicaErrorStatus = make(map[string]mariadbv1alpha1.ReplicaErrorStatus)
			}
			replicaErrorStatus[pod] = *mergedReplicaErrors
		}
	}
	return replicaErrorStatus, nil
}

func (r *MariaDBReconciler) getMaxScalePrimaryPod(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*int, error) {
	if !mdb.IsMaxScaleEnabled() {
		return nil, nil
	}
	mxs, err := r.RefResolver.MaxScale(ctx, mdb.Spec.MaxScaleRef, mdb.Namespace)
	if err != nil {
		return nil, err
	}
	primarySrv := mxs.Status.GetPrimaryServer()
	if primarySrv == nil {
		return nil, errors.New("MaxScale primary server not found")
	}
	podIndex, err := podIndexForServer(*primarySrv, mxs, mdb)
	if err != nil {
		return nil, fmt.Errorf("error getting Pod for MaxScale server '%s': %v", *primarySrv, err)
	}
	return podIndex, nil
}

func (r *MariaDBReconciler) setUpdatedCondition(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mdb), &sts); err != nil {
		return err
	}
	if sts.Status.UpdateRevision == "" {
		return nil
	}

	pods, err := mdbpod.ListMariaDBPods(ctx, r.Client, mdb)
	if err != nil {
		return fmt.Errorf("error listing Pods: %v", err)
	}

	podsUpdated := 0
	for _, pod := range pods {
		if mdbpod.PodUpdated(&pod, sts.Status.UpdateRevision) {
			podsUpdated++
		}
	}

	logger := log.FromContext(ctx)

	if podsUpdated >= int(sts.Status.Replicas) {
		logger.V(1).Info("MariaDB is up to date")
		condition.SetUpdated(&mdb.Status)
	} else if podsUpdated > 0 {
		logger.V(1).Info("MariaDB update in progress")
		condition.SetUpdating(&mdb.Status)
	} else {
		logger.V(1).Info("MariaDB has a pending update")
		condition.SetPendingUpdate(&mdb.Status)
	}
	return nil
}

func mergeReplicaErrors(current *mariadbv1alpha1.ReplicaErrorStatus,
	new *mariadbv1alpha1.ReplicaErrors) *mariadbv1alpha1.ReplicaErrorStatus {
	if new == nil {
		return current
	}
	now := metav1.Now()
	// First report of errors — initialize new status
	if current == nil {
		return &mariadbv1alpha1.ReplicaErrorStatus{
			ReplicaErrors:      *new,
			LastTransitionTime: now,
		}
	}
	// No state change
	if current.Equal(new) {
		return current
	}
	// Transition: healthy <-> error or changed error type
	return &mariadbv1alpha1.ReplicaErrorStatus{
		ReplicaErrors:      *new,
		LastTransitionTime: now,
	}
}

func podIndexForServer(serverName string, mxs *mariadbv1alpha1.MaxScale, mdb *mariadbv1alpha1.MariaDB) (*int, error) {
	var server *mariadbv1alpha1.MaxScaleServer
	for _, srv := range mxs.Spec.Servers {
		if serverName == srv.Name {
			server = &srv
			break
		}
	}
	if server == nil {
		return nil, fmt.Errorf("MaxScale server '%s' not found", serverName)
	}

	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		address := stspkg.PodFQDNWithService(mdb.ObjectMeta, i, mdb.InternalServiceKey().Name)
		if server.Address == address {
			return &i, nil
		}
	}
	return nil, fmt.Errorf("MariaDB Pod with address '%s' not found", server.Address)
}

func defaultPrimary(mdb *mariadbv1alpha1.MariaDB) {
	if mdb.Status.CurrentPrimaryPodIndex != nil || mdb.Status.CurrentPrimary != nil {
		return
	}
	podIndex := 0
	if mdb.IsGaleraEnabled() {
		galera := ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{})
		podIndex = ptr.Deref(galera.Primary.PodIndex, 0)
	}
	if mdb.IsReplicationEnabled() {
		replication := ptr.Deref(mdb.Spec.Replication, mariadbv1alpha1.Replication{})
		podIndex = ptr.Deref(replication.Primary.PodIndex, 0)
	}
	mdb.Status.CurrentPrimaryPodIndex = &podIndex
	mdb.Status.CurrentPrimary = ptr.To(stspkg.PodName(mdb.ObjectMeta, podIndex))
}

func setMaxScalePrimary(mdb *mariadbv1alpha1.MariaDB, podIndex *int) {
	if !mdb.IsMaxScaleEnabled() || podIndex == nil {
		return
	}
	mdb.Status.CurrentPrimaryPodIndex = podIndex
	mdb.Status.CurrentPrimary = ptr.To(stspkg.PodName(mdb.ObjectMeta, *podIndex))
}
