package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/health"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/v25/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	mdbtime "github.com/mariadb-operator/mariadb-operator/v25/pkg/time"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var errPhysicalBackupNoTargetPodsAvailable = errors.New("no target Pods available")

// PhysicalBackupReconciler reconciles a PhysicalBackup object
type PhysicalBackupReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	Recorder          record.EventRecorder
	Discovery         *discovery.Discovery
	RefResolver       *refresolver.RefResolver
	ConditionComplete *condition.Complete
	RBACReconciler    *rbac.RBACReconciler
	PVCReconciler     *pvc.PVCReconciler
	BackupProcessor   backup.BackupProcessor
}

//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;watch;create;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PhysicalBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var backup mariadbv1alpha1.PhysicalBackup
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mariadb, err := r.RefResolver.MariaDB(ctx, &backup.Spec.MariaDBRef, backup.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			r.ConditionComplete.PatcherRefResolver(err, mariadb)(status)
		})
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}
	if backup.Spec.MariaDBRef.WaitForIt && !mariadb.IsReady() {
		if err := r.patchStatus(ctx, &backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			r.ConditionComplete.PatcherFailed("MariaDB not ready")(status)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Backup: %v", err)
		}
		r.Recorder.Event(
			&backup,
			corev1.EventTypeWarning,
			mariadbv1alpha1.ReasonMariaDBNotReady,
			"Pausing backup: MariaDB not ready",
		)

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	logger := log.FromContext(ctx).
		WithName("physicalbackup").
		WithValues(
			"mariadb", mariadb.Name,
		)
	if !shouldReconcilePhysicalBackup(mariadb, logger) {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.setDefaults(ctx, &backup, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting PhysicalBackup: %v", err)
	}
	return r.reconcile(ctx, &backup, mariadb, logger)
}

func (r *PhysicalBackupReconciler) setDefaults(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) error {
	return r.patch(ctx, backup, func(b *mariadbv1alpha1.PhysicalBackup) {
		backup.SetDefaults(mariadb)
	})
}

func (r *PhysicalBackupReconciler) reconcile(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (ctrl.Result, error) {
	if backup.Spec.Storage.VolumeSnapshot != nil {
		return r.reconcileSnapshots(ctx, backup, mariadb, logger)
	}
	return r.reconcileJobs(ctx, backup, mariadb, logger)
}

type scheduleFn func(now time.Time, cronSchedule cron.Schedule) (ctrl.Result, error)

func (r *PhysicalBackupReconciler) reconcileTemplate(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	numReconciledObjects int, scheduleFn scheduleFn) (ctrl.Result, error) {
	if backup.Spec.Schedule != nil {
		return r.reconcileTemplateScheduled(ctx, backup, scheduleFn)
	}
	if numReconciledObjects == 0 {
		return scheduleFn(time.Now(), nil)
	}
	return ctrl.Result{}, nil
}

func (r *PhysicalBackupReconciler) reconcileTemplateScheduled(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	scheduleFn scheduleFn) (ctrl.Result, error) {
	schedule := ptr.Deref(backup.Spec.Schedule, mariadbv1alpha1.PhysicalBackupSchedule{})

	if schedule.Suspend {
		return ctrl.Result{}, nil
	}

	isImmediate := ptr.Deref(schedule.Immediate, true)
	now := time.Now()
	if isImmediate && backup.Status.LastScheduleCheckTime == nil {
		return scheduleFn(now, nil)
	}

	if schedule.Cron == "" {
		return ctrl.Result{}, nil
	}
	cronSchedule, err := mariadbv1alpha1.CronParser.Parse(schedule.Cron)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error parsing cron schedule: %v", err)
	}

	if backup.Status.LastScheduleCheckTime == nil {
		nextTime := cronSchedule.Next(now)

		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.LastScheduleCheckTime = &metav1.Time{
				Time: now,
			}
			status.NextScheduleTime = &metav1.Time{
				Time: nextTime,
			}
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching status: %v", err)
		}
		return ctrl.Result{RequeueAfter: nextTime.Sub(now)}, nil
	}

	nextTime := cronSchedule.Next(backup.Status.LastScheduleCheckTime.Time)

	if now.Before(nextTime) {
		return ctrl.Result{RequeueAfter: nextTime.Sub(now)}, nil
	}
	return scheduleFn(now, cronSchedule)
}

func (r *PhysicalBackupReconciler) patch(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	patcher func(*mariadbv1alpha1.PhysicalBackup)) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(backup)
	return r.Patch(ctx, backup, patch)
}

func (r *PhysicalBackupReconciler) patchStatus(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	patcher func(*mariadbv1alpha1.PhysicalBackupStatus)) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(&backup.Status)
	return r.Client.Status().Patch(ctx, backup, patch)
}

func (r *PhysicalBackupReconciler) physicalBackupTarget(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) (*int, error) {
	return physicalBackupTargetWithFuncs(ctx, backup, mariadb, r.primaryTarget, r.replicaTarget, logger)
}

func (r *PhysicalBackupReconciler) primaryTarget(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (*int, error) {
	if mariadb.Status.CurrentPrimary == nil || mariadb.Status.CurrentPrimaryPodIndex == nil {
		logger.V(1).Info("no target Pods Available: 'status.currentPrimary' and 'status.currentPrimaryPodIndex' must be set")
		return nil, errPhysicalBackupNoTargetPodsAvailable
	}
	key := types.NamespacedName{
		Name:      *mariadb.Status.CurrentPrimary,
		Namespace: mariadb.Namespace,
	}
	var pod corev1.Pod
	if err := r.Get(ctx, key, &pod); err != nil {
		return nil, fmt.Errorf("error getting primary Pod: %v", err)
	}
	if !mariadbpod.PodReady(&pod) {
		logger.V(1).Info("no target Pods Available: no healthy primary Pod available")
		return nil, errPhysicalBackupNoTargetPodsAvailable
	}
	podIndex := mariadb.Status.CurrentPrimaryPodIndex
	logger.Info("Using primary as PhysicalBackup target", "target-pod-index", podIndex)
	return podIndex, nil
}

func (r *PhysicalBackupReconciler) replicaTarget(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (*int, error) {
	podIndex, err := health.SecondaryPodHealthyIndex(ctx, r.Client, mariadb)
	if err != nil {
		if errors.Is(err, health.ErrNoHealthyInstancesAvailable) {
			logger.V(1).Info("no target Pods Available: no healthy secondary Pods available")
			return nil, errPhysicalBackupNoTargetPodsAvailable
		}
		return nil, fmt.Errorf("error getting target Pod index: %v", err)
	}
	logger.Info("Using replica as PhysicalBackup target", "target-pod-index", podIndex)
	return podIndex, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PhysicalBackupReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, opts controller.Options) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.PhysicalBackup{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		WithOptions(opts)
	if err := r.indexJobs(ctx, mgr); err != nil {
		return fmt.Errorf("error indexing PhysicalBackup Jobs: %v", err)
	}
	if err := r.watchSnapshots(ctx, builder); err != nil {
		return fmt.Errorf("error watching PhysicalBackup VolumeSnapshots: %v", err)
	}
	return builder.Complete(r)
}

func shouldReconcilePhysicalBackup(mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) bool {
	if mdb.IsSuspended() {
		logger.Info("MariaDB is suspended, skipping PhysicalBackup schedule...")
		return false
	}
	if mdb.IsRestoringBackup() {
		logger.Info("Backup restoration in progress, skipping PhysicalBackup schedule...")
		return false
	}
	if mdb.IsInitializing() {
		logger.Info("Initialization in progress, skipping PhysicalBackup schedule...")
		return false
	}
	if mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() {
		logger.Info("Switchover in progress, skipping PhysicalBackup schedule...")
		return false
	}
	if mdb.IsUpdating() || mdb.HasPendingUpdate() {
		logger.Info("Update in progress, skipping PhysicalBackup schedule...")
		return false
	}
	if mdb.IsResizingStorage() {
		logger.Info("Storage resize in progress, skipping PhysicalBackup schedule...")
		return false
	}
	if mdb.HasGaleraNotReadyCondition() {
		logger.Info("Galera not ready, skipping PhysicalBackup schedule...")
		return false
	}
	return true
}

func getObjectName(obj client.Object, now time.Time) string {
	return fmt.Sprintf("%s-%s", obj.GetName(), mdbtime.Format(now))
}

func parseObjectTime(obj client.Object) (time.Time, error) {
	return mariadbv1alpha1.ParsePhysicalBackupTime(obj.GetName())
}

func sortByObjectTime[T client.Object](objList []T) error {
	var parseErr error
	sort.Slice(objList, func(i, j int) bool {
		if parseErr != nil {
			return false
		}

		objTime, err := parseObjectTime(objList[i])
		if err != nil {
			parseErr = fmt.Errorf("error parsing object time: %v", err)
			return false
		}
		anotherObjTime, err := parseObjectTime(objList[j])
		if err != nil {
			parseErr = fmt.Errorf("error parsing object time: %v", err)
			return false
		}

		return objTime.After(anotherObjTime)
	})
	return parseErr
}

type targetFn func(context.Context, *mariadbv1alpha1.MariaDB, logr.Logger) (*int, error)

func physicalBackupTargetWithFuncs(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB, primaryFn, replicaFn targetFn, logger logr.Logger) (*int, error) {
	if !mariadb.IsHAEnabled() {
		return primaryFn(ctx, mariadb, logger)
	}
	target := ptr.Deref(backup.Spec.Target, mariadbv1alpha1.PhysicalBackupTargetReplica)
	switch target {
	case mariadbv1alpha1.PhysicalBackupTargetReplica:
		return replicaFn(ctx, mariadb, logger)
	case mariadbv1alpha1.PhysicalBackupTargetPreferReplica:
		podIndex, err := replicaFn(ctx, mariadb, logger)
		if err != nil && !errors.Is(err, errPhysicalBackupNoTargetPodsAvailable) {
			return nil, fmt.Errorf("error getting replica target: %v", err)
		}
		if podIndex != nil {
			return podIndex, nil
		}
		return primaryFn(ctx, mariadb, logger)
	}
	return nil, errPhysicalBackupNoTargetPodsAvailable
}
