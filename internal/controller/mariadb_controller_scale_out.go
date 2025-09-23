package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	mdbsnapshot "github.com/mariadb-operator/mariadb-operator/v26/pkg/volumesnapshot"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileScaleOut(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsReplicationEnabled() {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("scale-out")

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return ctrl.Result{}, err
	}

	isScalingOut, err := r.isScalingOut(mariadb, &sts)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !isScalingOut {
		if result, err := r.setScaledOutAndCleanup(ctx, mariadb, logger); !result.IsZero() || err != nil {
			return result, err
		}
		return ctrl.Result{}, nil
	}
	fromIndex := ptr.Deref(mariadb.Status.ScaleOutInitialIndex, int(sts.Status.Replicas))
	logger = logger.WithValues("from-index", fromIndex)

	if !mariadb.IsScalingOut() || mariadb.ScalingOutError() != nil {
		if result, err := r.reconcileScaleOutError(ctx, mariadb, fromIndex, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetScalingOut(status)
		status.ScaleOutInitialIndex = &fromIndex
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	physicalBackupKey := mariadb.PhysicalBackupScaleOutKey()

	if result, err := r.reconcileReplicaPhysicalBackup(ctx, physicalBackupKey, mariadb, logger); !result.IsZero() || err != nil {
		return result, err
	}
	physicalBackup, err := r.getPhysicalBackup(ctx, physicalBackupKey, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	snapshotKey, err := r.getVolumeSnapshotKey(ctx, mariadb, physicalBackup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot key: %v", err)
	}

	if result, err := r.reconcilePVCs(ctx, mariadb, fromIndex, snapshotKey, logger); !result.IsZero() || err != nil {
		return result, err
	}

	if physicalBackup.Spec.Storage.VolumeSnapshot == nil {
		replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
		bootstrapFrom := ptr.Deref(replication.Replica.ReplicaBootstrapFrom, mariadbv1alpha1.ReplicaBootstrapFrom{})

		if result, err := r.reconcileRollingInitJobs(
			ctx,
			mariadb,
			fromIndex,
			logger.WithName("job"),
			builder.WithPhysicalBackup(physicalBackup, time.Now(), bootstrapFrom.RestoreJob),
		); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) isScalingOut(mdb *mariadbv1alpha1.MariaDB, sts *appsv1.StatefulSet) (bool, error) {
	if !mdb.IsReplicationEnabled() || !mdb.HasConfiguredReplication() || sts.Status.Replicas == 0 {
		return false, nil
	}
	if mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() || mdb.IsInitializing() || mdb.IsRecoveringReplicas() ||
		mdb.IsRestoringBackup() || mdb.IsResizingStorage() || mdb.IsUpdating() {
		return false, nil
	}
	// user is able to rollback scale out operation at any point by matching the number of existing replicas
	if sts.Status.Replicas == mdb.Spec.Replicas {
		return false, nil
	}
	// ongoing scale out process
	if mdb.IsScalingOut() {
		return true, nil
	}
	// initial condition for starting scale out process, all replicas should be ready
	return sts.Status.Replicas == sts.Status.ReadyReplicas &&
		sts.Status.Replicas < mdb.Spec.Replicas, nil
}

func (r *MariaDBReconciler) reconcileScaleOutError(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, fromIndex int,
	logger logr.Logger) (ctrl.Result, error) {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})

	if replication.Replica.ReplicaBootstrapFrom == nil {
		r.Recorder.Eventf(mariadb, nil, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBScaleOutError, mariadbv1alpha1.ActionReconciling,
			"Unable to scale out MariaDB: replica datasource not found (replication.replica.bootstrapFrom is nil)")

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetScaleOutError(status, "replica datasource not found (replication.replica.bootstrapFrom is nil)")
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}

		logger.Info("Unable to scale out MariaDB: replica datasource not found (replication.replica.bootstrapFrom is nil). Requeuing...")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	pvcsAlreadyExist, err := r.pvcAlreadyExists(ctx, mariadb, fromIndex)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error checking PVCs: %v", err)
	}
	if pvcsAlreadyExist {
		r.Recorder.Eventf(mariadb, nil, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBScaleOutError, mariadbv1alpha1.ActionReconciling,
			"Unable to scale out MariaDB: storage PVCs already exist")

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetScaleOutError(status, "storage PVCs already exist")
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}

		logger.Info("Unable to scale out MariaDB: storage PVCs already exist. Requeuing...")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) pvcAlreadyExists(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, fromIndex int) (bool, error) {
	for i := fromIndex; i < int(mariadb.Spec.Replicas); i++ {
		pvcKey := mariadb.PVCKey(builder.StorageVolumeRole, i)
		var pvc corev1.PersistentVolumeClaim
		err := r.Get(ctx, pvcKey, &pvc)
		if err == nil {
			return true, nil
		}
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("error getting PVC %s: %v", pvcKey.Name, err)
		}
	}
	return false, nil
}

func (r *MariaDBReconciler) reconcileReplicaPhysicalBackup(ctx context.Context, key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		logger.Info("Current PhysicalBackup err!=nil", "err", err)
		if apierrors.IsNotFound(err) {
			logger.Info("Creating PhysicalBackup", "name", key.Name)
			if err := r.createReplicaPhysicalBackup(ctx, key, mariadb); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// If backup is already present but expired (backup age >  master binlog_retention) we need to destroy it to force a new backup
	var binlogExpireLogsDuration time.Duration
	var binlogExpireErr error
	var ageThreshold *time.Time
	replication := mariadb.Replication()

	if replication.IsExternalReplication() {
		emdb, err := r.RefResolver.ExternalMariaDB(ctx, &replication.ReplicaFromExternal.MariaDBRef.ObjectReference, mariadb.Namespace)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting external MariaDB: %v", err)
		}
		logger.Info("Getting the binlog_expire_logs_seconds on the external MariaDB")
		binlogExpireLogsDuration, binlogExpireErr = getBinlogExpireLogsDuration(emdb, ctx, r.RefResolver, logger)
	} else {
		logger.Info("Getting the binlog_expire_logs_seconds on primary MariaDB")
		binlogExpireLogsDuration, binlogExpireErr = getInternalBinlogExpireLogsDuration(mariadb, ctx, r.RefResolver)
	}

	if binlogExpireErr == nil && binlogExpireLogsDuration != 0 {
		ageThreshold = ptr.To(time.Now().Add(-binlogExpireLogsDuration))
	} else if binlogExpireErr != nil {
		// In case of failure to get the binlogExpireLogsDuration set ageThreshold do now to force a new backup
		logger.Info("Unable to get binlog_expire_logs_seconds, setting age threshold to now to force new backup", "error", binlogExpireErr)
		ageThreshold = ptr.To(time.Now())
	}

	if !physicalBackup.IsComplete() {
		if replication.IsExternalReplication() {
			backupStartTime := physicalBackup.CreationTimestamp.Time
			jobWaitTimeout, _ := time.ParseDuration("240s")
			jobs, err := job.ListJobs(ctx, r.Client, &physicalBackup)
			if (err != nil || len(jobs.Items) == 0) && time.Since(backupStartTime) > jobWaitTimeout {
				logger.Info("ExternalReplication, physical backup job launch wait timeout. Trigger logical backup")
				return ctrl.Result{RequeueAfter: 1 * time.Second}, errPhysicalBackupJobLaunchTimeout
			}
		}
		logger.V(1).Info("Replica PhysicalBackup job not completed. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	} else {
		if ageThreshold != nil && physicalBackup.Status.LastScheduleTime.Time.Before(*ageThreshold) {
			logger.Info("Existent backup is expired, destroying to create a new one")
			if err := r.cleanupPhysicalBackup(ctx, mariadb.PhysicalBackupReplicaRecoveryKey()); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}
	logger.V(1).Info("Replica PhysicalBackup completed.")
	return ctrl.Result{}, nil
}

func getInternalBinlogExpireLogsDuration(mdb *mariadbv1alpha1.MariaDB, ctx context.Context,
	refResolver *refresolver.RefResolver) (time.Duration, error) {
	var external_client *sql.Client
	var err error
	if external_client, err = sql.NewClientWithMariaDB(ctx, mdb, refResolver); err != nil {
		return time.Duration(0), fmt.Errorf("error getting external MariaDB client: %v", err)
	}
	defer external_client.Close()

	var binlogExpireLogsSecondsStr string
	var binlogExpireLogsSeconds int

	binlogExpireLogsSecondsStr, err = external_client.SystemVariable(ctx, "binlog_expire_logs_seconds")
	if err != nil {
		return time.Duration(0), fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
	}
	binlogExpireLogsSeconds, _ = strconv.Atoi(binlogExpireLogsSecondsStr)

	return time.Duration(binlogExpireLogsSeconds) * time.Second, nil
}

func (r *MariaDBReconciler) createReplicaPhysicalBackup(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB) error {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Replica.ReplicaBootstrapFrom == nil {
		return errors.New("replica datasource not found")
	}

	tplKey := types.NamespacedName{
		Name:      replication.Replica.ReplicaBootstrapFrom.PhysicalBackupTemplateRef.Name,
		Namespace: mariadb.Namespace,
	}
	var tpl mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, tplKey, &tpl); err != nil {
		return fmt.Errorf("error getting PhysicalBackup template: %v", err)
	}

	physicalBackup, err := r.Builder.BuildReplicaRecoveryPhysicalBackup(key, &tpl, mariadb)
	if err != nil {
		return fmt.Errorf("error building PhysicalBackup: %v", err)
	}
	return r.Create(ctx, physicalBackup)
}

func (r *MariaDBReconciler) getPhysicalBackup(ctx context.Context, key types.NamespacedName,
	mariadb *mariadbv1alpha1.MariaDB) (*mariadbv1alpha1.PhysicalBackup, error) {
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		return nil, err
	}
	return &physicalBackup, nil
}

func (r *MariaDBReconciler) getVolumeSnapshotKey(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	physicalBackup *mariadbv1alpha1.PhysicalBackup) (*types.NamespacedName, error) {
	if physicalBackup.Spec.Storage.VolumeSnapshot == nil {
		return nil, nil
	}
	snapshotList, err := mdbsnapshot.ListVolumeSnapshots(ctx, r.Client, physicalBackup)
	if err != nil {
		return nil, err
	}
	if len(snapshotList.Items) == 0 {
		return nil, errors.New("VolumeSnapshot not found")
	}
	sort.Slice(snapshotList.Items, func(i, j int) bool {
		return snapshotList.Items[i].CreationTimestamp.After(snapshotList.Items[j].CreationTimestamp.Time)
	})
	return ptr.To(client.ObjectKeyFromObject(&snapshotList.Items[0])), nil
}

func (r *MariaDBReconciler) setScaledOutAndCleanup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Scale out and cleanup")
	if !mariadb.IsScalingOut() {
		logger.Info("Not scaling out")
		return ctrl.Result{}, nil
	}
	physicalBackupKey := mariadb.PhysicalBackupScaleOutKey()
	logger.Info("physical backup", "key", physicalBackupKey)

	if mariadb.Status.ScaleOutInitialIndex != nil {
		logger.Info("Scale out initial index", "index", *mariadb.Status.ScaleOutInitialIndex)
		fromIndex := *mariadb.Status.ScaleOutInitialIndex

		physicalBackup, err := r.getPhysicalBackup(ctx, physicalBackupKey, mariadb)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
		}
		snapshotKey, err := r.getVolumeSnapshotKey(ctx, mariadb, physicalBackup)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot key: %v", err)
		}

		if err := r.ensureReplicationConfigured(ctx, fromIndex, mariadb, snapshotKey, logger); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			status.ScaleOutInitialIndex = nil
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}
		// Requeue to track replication status
		if mariadb.IsReplicationEnabled() {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetScaledOut(status)
		status.ScaleOutInitialIndex = nil
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if err := r.cleanupPhysicalBackup(ctx, physicalBackupKey); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.cleanupInitJobs(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) cleanupPhysicalBackup(ctx context.Context, key types.NamespacedName) error {
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, &physicalBackup)
}
