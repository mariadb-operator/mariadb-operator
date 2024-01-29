package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mdb), &sts); err != nil {
		return ctrl.Result{}, err
	}

	replicationStatus, err := r.getReplicationStatus(ctx, mdb)
	if err != nil {
		log.FromContext(ctx).V(1).Info("error getting replication status", "err", err)
	}

	return ctrl.Result{}, r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.Replicas = sts.Status.ReadyReplicas
		status.FillWithDefaults(mdb)

		if replicationStatus != nil {
			status.ReplicationStatus = replicationStatus
		}

		if mdb.IsRestoringBackup() || mdb.IsSwitchingPrimary() || mdb.HasGaleraNotReadyCondition() {
			return nil
		}
		condition.SetReadyWithStatefulSet(&mdb.Status, &sts)
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

	var replicationStatus mariadbv1alpha1.ReplicationStatus
	logger := replLogger(ctx)
	for i := 0; i < int(mdb.Spec.Replicas); i++ {
		pod := stsobj.PodName(mdb.ObjectMeta, i)

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
		replicationStatus = append(replicationStatus, mariadbv1alpha1.PodReplicationState{
			Pod:   pod,
			State: state,
		})
	}
	return replicationStatus, nil
}

func replLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("replication")
}
