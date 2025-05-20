package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
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
	snapshotList, err := r.listSnapshots(ctx, backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing VolumeSnapshots: %v", err)
	}
	if err := r.reconcileSnapshotStatus(ctx, backup, snapshotList); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling status: %v", err)
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
		client.MatchingLabels(
			labels.NewLabelsBuilder().
				WithPhysicalBackupSelectorLabels(backup).
				Build(),
		),
	); err != nil {
		return nil, err
	}
	return &volumeSnapshotList, nil
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
				"label", labels.PhysicalBackupName,
			).
			Info("Watching labeled VolumeSnapshots")
		builder.Watches(
			&volumesnapshotv1.VolumeSnapshot{},
			handler.EnqueueRequestsFromMapFunc(r.mapVolumeSnapshotsToRequests),
			ctrlbuilder.WithPredicates(
				predicate.PredicateWithLabel(labels.PhysicalBackupName),
			),
		)
	}
	return nil
}

func (r *PhysicalBackupReconciler) mapVolumeSnapshotsToRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	physicalBackupName, ok := obj.GetLabels()[labels.PhysicalBackupName]
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

		if err := status.Error; err != nil {
			message := ptr.Deref(err.Message, "Error")

			if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
				status.SetCondition(metav1.Condition{
					Type:    mariadbv1alpha1.ConditionTypeComplete,
					Status:  metav1.ConditionTrue,
					Reason:  mariadbv1alpha1.ConditionReasonJobFailed,
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
	} else if len(snapshotList.Items) > 0 && numReady > 0 {
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
