package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	mdbsnapshot "github.com/mariadb-operator/mariadb-operator/v25/pkg/volumesnapshot"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *PhysicalBackupReconciler) reconcileSnapshots(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	exist, err := r.Discovery.VolumeSnapshotExist()
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exist {
		r.Recorder.Event(backup, corev1.EventTypeWarning, mariadbv1alpha1.ReasonCRDNotFound,
			"Unable to reconcile PhysicalBackup: VolumeSnapshot CRD not installed in the cluster")
		log.FromContext(ctx).Error(errors.New("VolumeSnapshot CRD not installed in the cluster"), "Unable to reconcile PhysicalBackup")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	snapshotList, err := mdbsnapshot.ListVolumeSnapshots(ctx, r.Client, backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing VolumeSnapshots: %v", err)
	}
	if err := r.reconcileSnapshotStatus(ctx, backup, snapshotList); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling status: %v", err)
	}
	if err := r.cleanupSnapshots(ctx, backup, snapshotList); err != nil {
		return ctrl.Result{}, fmt.Errorf("error cleaning up Jobs: %v", err)
	}
	if result, err := r.waitForInProgressSnapshots(ctx, backup, snapshotList); !result.IsZero() || err != nil {
		return result, err
	}

	return r.reconcileTemplate(ctx, backup, len(snapshotList.Items), func(now time.Time, cronSchedule cron.Schedule) (ctrl.Result, error) {
		return r.scheduleSnapshot(ctx, backup, mariadb, now, cronSchedule)
	})
}

func (r *PhysicalBackupReconciler) watchSnapshots(ctx context.Context, builder *ctrlbuilder.Builder) error {
	volumeSnapshotExists, err := r.Discovery.VolumeSnapshotExist()
	if err != nil {
		return fmt.Errorf("error discovering VolumeSnapshot: %v", err)
	}
	if volumeSnapshotExists {
		log.FromContext(ctx).
			WithName("watcher").
			WithValues(
				"kind", "VolumeSnapshot",
				"label", metadata.PhysicalBackupNameLabel,
			).
			Info("Watching labeled VolumeSnapshots")
		builder.Watches(
			&volumesnapshotv1.VolumeSnapshot{},
			handler.EnqueueRequestsFromMapFunc(r.mapVolumeSnapshotsToRequests),
			ctrlbuilder.WithPredicates(
				predicate.PredicateWithLabel(metadata.PhysicalBackupNameLabel),
			),
		)
	}
	return nil
}

func (r *PhysicalBackupReconciler) mapVolumeSnapshotsToRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	physicalBackupName, ok := obj.GetLabels()[metadata.PhysicalBackupNameLabel]
	if !ok {
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      physicalBackupName,
				Namespace: obj.GetNamespace(),
			},
		},
	}
}

func (r *PhysicalBackupReconciler) reconcileSnapshotStatus(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	snapshotList *volumesnapshotv1.VolumeSnapshotList) error {
	logger := log.FromContext(ctx).WithName("status").V(1)
	schedule := ptr.Deref(backup.Spec.Schedule, mariadbv1alpha1.PhysicalBackupSchedule{})

	if schedule.Suspend {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonSnapshotSuspended,
				Message: "Suspended",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
		return nil
	}

	numReady := 0
	for _, snapshot := range snapshotList.Items {
		status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
		ready := ptr.Deref(status.ReadyToUse, false)

		if status.Error != nil {
			message := ptr.Deref(status.Error.Message, "Error")

			if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
				status.SetCondition(metav1.Condition{
					Type:    mariadbv1alpha1.ConditionTypeComplete,
					Status:  metav1.ConditionFalse,
					Reason:  mariadbv1alpha1.ConditionReasonSnapshotFailed,
					Message: message,
				})
			}); err != nil {
				logger.Info("error patching status", "err", err)
			}
			return nil
		} else if ready {
			numReady++
		}
	}

	if len(snapshotList.Items) > 0 && numReady == len(snapshotList.Items) {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonSnapshotComplete,
				Message: "Success",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	} else if len(snapshotList.Items) > 0 {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonSnapshotInProgress,
				Message: "In progress",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	} else {
		message := "Not complete"
		if backup.Spec.Schedule != nil {
			message = "Scheduled"
		}

		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonSnapshotNotComplete,
				Message: message,
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	}

	return nil
}

func (r *PhysicalBackupReconciler) cleanupSnapshots(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	snapshotList *volumesnapshotv1.VolumeSnapshotList) error {
	if backup.Spec.Schedule == nil {
		return nil
	}

	var readySnapshotNames []string
	for _, snapshot := range snapshotList.Items {
		if mdbsnapshot.IsVolumeSnapshotReady(&snapshot) {
			readySnapshotNames = append(readySnapshotNames, snapshot.Name)
		}
	}
	maxRetention := backup.Spec.MaxRetention
	if maxRetention == (metav1.Duration{}) {
		maxRetention = mariadbv1alpha1.DefaultPhysicalBackupMaxRetention
	}
	logger := log.FromContext(ctx).WithName("snapshot")

	oldSnapshotNames := r.BackupProcessor.GetOldBackupFiles(readySnapshotNames, maxRetention.Duration, logger)
	for _, snapshotName := range oldSnapshotNames {
		key := types.NamespacedName{
			Name:      snapshotName,
			Namespace: backup.Namespace,
		}
		var snapshot volumesnapshotv1.VolumeSnapshot
		if err := r.Get(ctx, key, &snapshot); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("error getting VolumeSnapshot \"%s\": %v", key.Name, err)
		}

		err := r.Delete(ctx, &snapshot)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting VolumeSnapshot \"%s\": %v", snapshot.Name, err)
		}
		logger.V(1).Info("Deleted old Snapshot", "snapshot", key.Name, "physicalbackup", backup.Name)
	}

	return nil
}

func (r *PhysicalBackupReconciler) scheduleSnapshot(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, now time.Time, schedule cron.Schedule) (ctrl.Result, error) {
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
		status.LastScheduleCheckTime = &metav1.Time{
			Time: now,
		}
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

	return ctrl.Result{}, nil
}

func (r *PhysicalBackupReconciler) createVolumeSnapshot(ctx context.Context, snapshotKey types.NamespacedName,
	backup *mariadbv1alpha1.PhysicalBackup, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return errors.New("CurrentPrimaryPodIndex must be set")
	}
	podIndex := *mariadb.Status.CurrentPrimaryPodIndex
	logger := log.FromContext(ctx).
		WithName("snapshot").
		WithValues(
			"mariadb", mariadb.Name,
			"pod-index", podIndex,
		)

	client, err := sql.NewInternalClientWithPodIndex(ctx, mariadb, r.RefResolver, podIndex)
	if err != nil {
		return fmt.Errorf("error getting SQL client: %v", err)
	}
	defer client.Close()

	logger.V(1).Info("Locking tables with read lock")
	if err := client.LockTablesWithReadLock(ctx); err != nil {
		return fmt.Errorf("error locking tables with read lock: %v", err)
	}
	defer func() {
		logger.V(1).Info("Unlocking tables with read lock")
		if err := client.UnlockTables(ctx); err != nil {
			logger.Error(err, "error unlocking tables")
		}
	}()

	primaryPvcKey := mariadb.PVCKey(builder.StorageVolume, *mariadb.Status.CurrentPrimaryPodIndex)
	desiredSnapshot, err := r.Builder.BuildVolumeSnapshot(snapshotKey, backup, primaryPvcKey)
	if err != nil {
		return fmt.Errorf("error building VolumeSnapshot: %v", err)
	}

	var snapshot volumesnapshotv1.VolumeSnapshot
	if err = r.Get(ctx, snapshotKey, &snapshot); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting VolumeSnapshot: %v", err)
		}
		if err := r.Create(ctx, desiredSnapshot); err != nil {
			return fmt.Errorf("error creating VolumeSnapshot: %v", err)
		}
		r.Recorder.Eventf(
			backup,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonVolumeSnapshotCreated,
			"VolumeSnapshot %s scheduled",
			desiredSnapshot.Name,
		)
	}
	return nil // TODO: handle already exists
}

func (r *PhysicalBackupReconciler) waitForInProgressSnapshots(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	snapshotList *volumesnapshotv1.VolumeSnapshotList) (ctrl.Result, error) {
	for _, snapshot := range snapshotList.Items {
		if !mdbsnapshot.IsVolumeSnapshotReady(&snapshot) {
			if backup.Spec.Timeout != nil && !snapshot.CreationTimestamp.IsZero() &&
				time.Since(snapshot.CreationTimestamp.Time) > backup.Spec.Timeout.Duration {

				log.FromContext(ctx).Info("PhysicalBackup VolumeSnapshot timed out. Deleting...", "snapshot", snapshot.Name)
				if err := r.Delete(ctx, &snapshot); err != nil {
					return ctrl.Result{}, fmt.Errorf("error deleting expired VolumeSnapshot: %v", err)
				}
				return ctrl.Result{Requeue: true}, nil
			}

			status := ptr.Deref(snapshot.Status, volumesnapshotv1.VolumeSnapshotStatus{})
			log.FromContext(ctx).V(1).Info(
				"PhysicalBackup VolumeSnapshot is not ready. Requeuing...",
				"snapshot", snapshot.Name,
				"ready", ptr.Deref(status.ReadyToUse, false),
				"error", status.Error,
			)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}
	return ctrl.Result{}, nil
}
