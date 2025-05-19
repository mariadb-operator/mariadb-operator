package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func (r *PhysicalBackupReconciler) reconcileSnapshots(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	snapshotList, err := r.listSnapshots(ctx, backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing VolumeSnapshots: %v", err)
	}
	return r.reconcileTemplate(ctx, backup, len(snapshotList.Items), func(now time.Time, cronSchedule cron.Schedule) (ctrl.Result, error) {
		return r.scheduleSnapshot(ctx, backup, mariadb, snapshotList, now, cronSchedule)
	})
}

func (r *PhysicalBackupReconciler) listSnapshots(ctx context.Context,
	backup *mariadbv1alpha1.PhysicalBackup) (*volumesnapshotv1.VolumeSnapshotList, error) {
	var volumeSnapshotList volumesnapshotv1.VolumeSnapshotList
	if err := r.List(
		ctx,
		&volumeSnapshotList,
		client.InNamespace(backup.Namespace),
		client.MatchingFields{metaCtrlFieldPath: backup.Name},
	); err != nil {
		return nil, err
	}
	return &volumeSnapshotList, nil
}

func (r *PhysicalBackupReconciler) indexSnapshots(ctx context.Context, mgr manager.Manager) error {
	log.FromContext(ctx).
		WithName("indexer").
		WithValues(
			"kind", "VolumeSnapshot",
			"field", metaCtrlFieldPath,
		).
		Info("Watching field")
	return mgr.GetFieldIndexer().IndexField(
		ctx,
		&volumesnapshotv1.VolumeSnapshot{},
		metaCtrlFieldPath,
		func(o client.Object) []string {
			volumeSnapshot, ok := o.(*volumesnapshotv1.VolumeSnapshot)
			if !ok {
				return nil
			}
			owner := metav1.GetControllerOf(volumeSnapshot)
			if owner == nil {
				return nil
			}
			if owner.Kind != mariadbv1alpha1.PhysicalBackupKind {
				return nil
			}
			if owner.APIVersion != mariadbv1alpha1.GroupVersion.String() {
				return nil
			}
			return []string{owner.Name}
		})
}

func (r *PhysicalBackupReconciler) scheduleSnapshot(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, snapshotList *volumesnapshotv1.VolumeSnapshotList,
	now time.Time, schedule cron.Schedule) (ctrl.Result, error) {
	if result, err := r.waitForReadySnapshots(ctx, snapshotList); !result.IsZero() || err != nil {
		return result, err
	}
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		log.FromContext(ctx).V(1).Info("Current primary not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	snapshotKey := types.NamespacedName{
		Name:      getObjectName(backup, now),
		Namespace: mariadb.Namespace,
	}
	if err := r.createVolumeSnapshot(ctx, snapshotKey, backup, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating VolumeSnapshot: %v", err)
	}

	if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
		status.LastScheduleTime = &metav1.Time{
			Time: now,
		}
		if schedule != nil {
			status.NextScheduleTime = &metav1.Time{
				Time: schedule.Next(now),
			}
		}
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
	}
	r.Recorder.Eventf(
		backup,
		corev1.EventTypeNormal,
		mariadbv1alpha1.ReasonVolumeSnapshotCreated,
		"VolumeSnapthot %s created",
		snapshotKey.Name,
	)

	return ctrl.Result{}, nil
}

func (r *PhysicalBackupReconciler) createVolumeSnapshot(ctx context.Context, snapshotKey types.NamespacedName,
	backup *mariadbv1alpha1.PhysicalBackup, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("CurrentPrimaryPodIndex must be set")
	}
	client, err := sql.NewClientWithMariaDB(ctx, mariadb, r.RefResolver)
	if err != nil {
		return fmt.Errorf("error getting SQL client: %v", err)
	}
	defer client.Close()

	if err := client.LockTablesWithReadLock(ctx); err != nil {
		return fmt.Errorf("error locking with read lock: %v", err)
	}
	defer func() {
		if err := client.UnlockTables(ctx); err != nil {
			log.FromContext(ctx).Error(err, "error unlocking tables")
		}
	}()

	primaryPvcKey := mariadb.PVCKey(builder.StorageVolume, *mariadb.Status.CurrentPrimaryPodIndex)
	desiredSnapshot, err := r.Builder.BuildVolumeSnapshot(snapshotKey, backup, primaryPvcKey)
	if err != nil {
		return fmt.Errorf("error building VolumeSnapshot: %v", err)
	}

	var snapshot volumesnapshotv1.VolumeSnapshot
	if err = r.Get(ctx, snapshotKey, &snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, desiredSnapshot)
		}
		return fmt.Errorf("error getting VolumeSnapshot: %v", err)
	}
	return nil // TODO: handle already exists
}

func (r *PhysicalBackupReconciler) waitForReadySnapshots(ctx context.Context,
	snapthotList *volumesnapshotv1.VolumeSnapshotList) (ctrl.Result, error) {
	for _, snapshot := range snapthotList.Items {
		if !isSnapshotReady(&snapshot) {
			status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
			log.FromContext(ctx).Info(
				"PhysicalBackup VolumeSnapshot is not ready. Requeuing...",
				"snapshot", snapshot.Name,
				"ready", ptr.Deref(status.ReadyToUse, false),
				"error", status.Error,
			)
			return ctrl.Result{Requeue: true}, nil
		}
	}
	return ctrl.Result{}, nil
}

func isSnapshotReady(snapshot *volumesnapshotv1.VolumeSnapshot) bool {
	status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
	ready := ptr.Deref(status.ReadyToUse, false)

	return ready && status.Error == nil
}
