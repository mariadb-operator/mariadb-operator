package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	mdbsnapshot "github.com/mariadb-operator/mariadb-operator/v25/pkg/volumesnapshot"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileScaleOut(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
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
		if err := r.setScaledOutAndCleanup(ctx, mariadb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	fromIndex := int(sts.Status.Replicas) // start one index after the last replica

	if !mariadb.IsScalingOut() || mariadb.ScalingOutError() != nil {
		if result, err := r.reconcileScaleOutError(ctx, mariadb, fromIndex, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetScalingOut(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if result, err := r.reconcilePhysicalBackup(ctx, mariadb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	physicalBackup, err := r.getScaleOutPhysicalBackup(ctx, mariadb)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	snapshotKey, err := r.getVolumeSnapshotKey(ctx, mariadb, physicalBackup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting VolumeSnapshot key: %v", err)
	}

	if result, err := r.reconcilePVCs(ctx, mariadb, fromIndex, snapshotKey); !result.IsZero() || err != nil {
		return result, err
	}

	if physicalBackup.Spec.Storage.VolumeSnapshot == nil {
		replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
		bootstrapFrom := ptr.Deref(replication.Replica.ReplicaBootstrapFrom, mariadbv1alpha1.ReplicaBootstrapFrom{})

		if result, err := r.reconcileRollingInitJobs(
			ctx,
			mariadb,
			fromIndex,
			withRestoreOpts(
				builder.WithPhysicalBackup(physicalBackup, time.Now(), bootstrapFrom.RestoreJob),
			),
			withPodInitializedFn(func(podIndex int) error {
				// last replica
				if podIndex+1 == int(mariadb.Spec.Replicas) {
					logger.Info("Adding replicas to configure")
					// if err := r.addReplicasToConfigure(ctx, mariadb, fromIndex, logger); err != nil {
					// 	return fmt.Errorf("error adding replicas to configure: %v", err)
					// }
				}
				return nil
			}),
		); !result.IsZero() || err != nil {
			return result, err
		}
	} else {
		logger.Info("Adding replicas to configure")
		// if err := r.addReplicasToConfigure(ctx, mariadb, fromIndex, logger); err != nil {
		// 	return ctrl.Result{}, err
		// }
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) isScalingOut(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, sts *appsv1.StatefulSet) (bool, error) {
	if !mariadb.IsReplicationEnabled() {
		return false, nil
	}
	return sts.Status.Replicas > 0 &&
		sts.Status.Replicas == sts.Status.ReadyReplicas &&
		sts.Status.Replicas < mariadb.Spec.Replicas, nil
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
		if err := r.Get(ctx, pvcKey, &pvc); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return false, fmt.Errorf("error getting PVC: %v", err)
		}
		return true, fmt.Errorf("PVC %s already exists", pvcKey.Name)
	}
	return false, nil
}

func (r *MariaDBReconciler) reconcilePhysicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	key := mariadb.PhysicalBackupScaleOutKey()
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.createPhysicalBackup(ctx, mariadb); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	if !physicalBackup.IsComplete() {
		logger.V(1).Info("PhysicalBackup init job not completed. Requeuing")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) createPhysicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Replica.ReplicaBootstrapFrom == nil {
		return errors.New("replica datasource not found")
	}

	templateKey := types.NamespacedName{
		Name:      replication.Replica.ReplicaBootstrapFrom.PhysicalBackupTemplateRef.Name,
		Namespace: mariadb.Namespace,
	}
	var physicalBackupTpl mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, templateKey, &physicalBackupTpl); err != nil {
		return fmt.Errorf("error getting PhysicalBackup template: %v", err)
	}

	physicalBackup := mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupScaleOutKey().Name,
			Namespace: mariadb.Namespace,
		},
		Spec: physicalBackupTpl.Spec,
	}
	physicalBackup.Spec.MariaDBRef = mariadbv1alpha1.MariaDBRef{
		ObjectReference: mariadbv1alpha1.ObjectReference{
			Name: mariadb.Name,
		},
	}
	physicalBackup.Spec.Schedule = &mariadbv1alpha1.PhysicalBackupSchedule{
		Immediate: ptr.To(true),
	}
	if err := controllerutil.SetControllerReference(mariadb, &physicalBackup, r.Scheme); err != nil {
		return fmt.Errorf("error setting controller reference to PhysicalBackup: %v", err)
	}
	return r.Create(ctx, &physicalBackup)
}

func (r *MariaDBReconciler) getScaleOutPhysicalBackup(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) (*mariadbv1alpha1.PhysicalBackup, error) {
	key := mariadb.PhysicalBackupScaleOutKey()
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
	return ptr.To(client.ObjectKeyFromObject(&snapshotList.Items[0])), nil
}

func (r *MariaDBReconciler) setScaledOutAndCleanup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !mariadb.IsScalingOut() {
		return nil
	}
	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetScaledOut(status)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if err := r.cleanupPhysicalBackup(ctx, mariadb); err != nil {
		return err
	}
	if err := r.cleanupInitJobs(ctx, mariadb, 0); err != nil {
		return err
	}
	return nil
}

func (r *MariaDBReconciler) cleanupPhysicalBackup(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := mariadb.PhysicalBackupScaleOutKey()
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, key, &physicalBackup); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, &physicalBackup)
}
