package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	mdbsnapshot "github.com/mariadb-operator/mariadb-operator/v25/pkg/volumesnapshot"
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

	isScalingOut, err := r.isScalingOut(ctx, mariadb, &sts)
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

func (r *MariaDBReconciler) isScalingOut(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, sts *appsv1.StatefulSet) (bool, error) {
	if !mdb.IsReplicationEnabled() || !mdb.HasConfiguredReplication() || sts.Status.Replicas == 0 {
		return false, nil
	}
	if mdb.IsSwitchingPrimary() || mdb.IsSwitchoverRequired() || mdb.IsInitializing() || mdb.IsRecoveringReplicas() ||
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
		r.Recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBScaleOutError,
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
		r.Recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBScaleOutError,
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
		if apierrors.IsNotFound(err) {
			logger.Info("Creating PhysicalBackup", "name", key.Name)
			if err := r.createReplicaPhysicalBackup(ctx, key, mariadb); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if !physicalBackup.IsComplete() {
		logger.V(1).Info("Replica PhysicalBackup job not completed. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
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

	physicalBackup, err := r.Builder.BuildPhysicalBackup(key, &tpl, mariadb)
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
	if !mariadb.IsScalingOut() {
		return ctrl.Result{}, nil
	}
	physicalBackupKey := mariadb.PhysicalBackupScaleOutKey()

	if mariadb.Status.ScaleOutInitialIndex != nil {
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
