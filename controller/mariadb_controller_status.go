package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	conditions "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	jobpkg "github.com/mariadb-operator/mariadb-operator/pkg/job"
	podpkg "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	stspkg "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mdb), &sts); err != nil {
		log.FromContext(ctx).V(1).Info("error getting StatefulSet", "err", err)
	}

	replicationStatus, replErr := r.getReplicationStatus(ctx, mdb)
	if replErr != nil {
		log.FromContext(ctx).V(1).Info("error getting replication status", "err", replErr)
	}
	mxsPrimaryPodIndex, mxsErr := r.getMaxScalePrimaryPod(ctx, mdb)
	if mxsErr != nil {
		log.FromContext(ctx).V(1).Info("error getting MaxScale primary Pod", "err", mxsErr)
	}
	var initJob *batchv1.Job
	if mdb.IsGaleraEnabled() && !mdb.HasGaleraConfiguredCondition() {
		var err error
		initJob, err = r.getInitJob(ctx, mdb)
		if err != nil {
			log.FromContext(ctx).V(1).Info("error getting init Job", "err", err)
		}
	}

	return ctrl.Result{}, r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.Replicas = sts.Status.ReadyReplicas
		defaultPrimary(mdb)
		setMaxScalePrimary(mdb, mxsPrimaryPodIndex)

		if replicationStatus != nil {
			status.ReplicationStatus = replicationStatus
		}

		if apierrors.IsNotFound(mxsErr) && !ptr.Deref(mdb.Spec.MaxScale, mariadbv1alpha1.MariaDBMaxScaleSpec{}).Enabled {
			r.ConditionReady.PatcherRefResolver(mxsErr, mariadbv1alpha1.MaxScale{})(&mdb.Status)
			return nil
		}
		if initJob != nil && !jobpkg.IsJobComplete(initJob) {
			condition.SetReadyWithInitJob(&mdb.Status, initJob)
			return nil
		}
		if mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsSwitchingPrimary() || mdb.HasGaleraNotReadyCondition() {
			return nil
		}

		if err := r.setUpdatedCondition(ctx, mdb); err != nil {
			return err
		}

		condition.SetReadyWithMariaDB(&mdb.Status, &sts, mdb)
		return nil
	})
}

func (r *MariaDBReconciler) getReplicationStatus(ctx context.Context,
	mdb *mariadbv1alpha1.MariaDB) (mariadbv1alpha1.ReplicationStatus, error) {
	if !mdb.Replication().Enabled {
		return nil, nil
	}

	clientSet, err := replication.NewReplicationClientSet(mdb, r.RefResolver)
	if err != nil {
		return nil, fmt.Errorf("error creating mariadb clientset: %v", err)
	}
	defer clientSet.Close()

	replicationStatus := make(mariadbv1alpha1.ReplicationStatus)
	logger := log.FromContext(ctx)
	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		pod := stspkg.PodName(mdb.ObjectMeta, i)

		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			logger.V(1).Info("error getting client for Pod", "err", err, "pod", pod)
			continue
		}

		var aggErr *multierror.Error

		masterEnabled, err := client.IsSystemVariableEnabled(ctx, "rpl_semi_sync_master_enabled")
		aggErr = multierror.Append(aggErr, err)
		slaveEnabled, err := client.IsSystemVariableEnabled(ctx, "rpl_semi_sync_slave_enabled")
		aggErr = multierror.Append(aggErr, err)

		if err := aggErr.ErrorOrNil(); err != nil {
			logger.V(1).Info("error checking Pod replication state", "err", err, "pod", pod)
			continue
		}

		state := mariadbv1alpha1.ReplicationStateNotConfigured
		if masterEnabled {
			state = mariadbv1alpha1.ReplicationStateMaster
		} else if slaveEnabled {
			state = mariadbv1alpha1.ReplicationStateSlave
		}
		replicationStatus[pod] = state
	}
	return replicationStatus, nil
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

func (r *MariaDBReconciler) getInitJob(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*batchv1.Job, error) {
	var job batchv1.Job
	if err := r.Get(ctx, mdb.InitKey(), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *MariaDBReconciler) setUpdatedCondition(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mdb)
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		return err
	}

	stsUpdateRevision := sts.Status.UpdateRevision
	if stsUpdateRevision == "" {
		return nil
	}

	list := corev1.PodList{}
	listOpts := &client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mdb).
				Build(),
		),
		Namespace: mdb.GetNamespace(),
	}
	if err := r.List(ctx, &list, listOpts); err != nil {
		return fmt.Errorf("error listing Pods: %v", err)
	}

	podsUpdated := 0
	for _, pod := range list.Items {
		if podpkg.PodUpdated(&pod, stsUpdateRevision) {
			podsUpdated++
		}
	}
	log.FromContext(ctx).V(1).Info("Pods updated", "pods", podsUpdated)

	if podsUpdated >= int(mdb.Spec.Replicas) {
		condition.SetUpdated(&mdb.Status)
	} else if podsUpdated > 0 {
		conditions.SetUpdating(&mdb.Status)
	} else {
		conditions.SetPendingUpdate(&mdb.Status)
	}
	return nil
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
	if mdb.Replication().Enabled {
		primaryReplication := ptr.Deref(mdb.Replication().Primary, mariadbv1alpha1.PrimaryReplication{})
		podIndex = ptr.Deref(primaryReplication.PodIndex, 0)
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
