package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	mdbtime "github.com/mariadb-operator/mariadb-operator/pkg/time"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const metaCtrlFieldPath = ".metadata.controller"

// PhysicalBackupReconciler reconciles a PhysicalBackup object
type PhysicalBackupReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Builder   *builder.Builder
	Recorder  record.EventRecorder
	Discovery *discovery.Discovery

	RefResolver       *refresolver.RefResolver
	ConditionComplete *condition.Complete
	RBACReconciler    *rbac.RBACReconciler
	PVCReconciler     *pvc.PVCReconciler
}

// +kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.mariadb.com,resources=physicalbackups/finalizers,verbs=update

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

	if err := r.setDefaults(ctx, &backup, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting PhysicalBackup: %v", err)
	}
	return r.reconcile(ctx, &backup, mariadb)
}

func (r *PhysicalBackupReconciler) setDefaults(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) error {
	return r.patch(ctx, backup, func(b *mariadbv1alpha1.PhysicalBackup) {
		backup.SetDefaults(mariadb)
	})
}

func (r *PhysicalBackupReconciler) reconcile(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {

	if backup.Spec.Storage.VolumeSnapshot != nil {
		// TODO: support for VolumeSnapshot
		return ctrl.Result{}, nil
	}
	return r.reconcileJobs(ctx, backup, mariadb)
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
	isImmediate := ptr.Deref(schedule.Immediate, false)
	cronSchedule, err := mariadbv1alpha1.CronParser.Parse(schedule.Cron)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error parsing cron schedule: %v", err)
	}

	now := time.Now()
	beforeNext := time.Now()
	if backup.Status.LastScheduleTime != nil {
		beforeNext = backup.Status.LastScheduleTime.Time
	}
	nextTime := cronSchedule.Next(beforeNext)

	if isImmediate && backup.Status.LastScheduleTime == nil {
		return scheduleFn(now, cronSchedule)
	}
	if backup.Status.LastScheduleTime == nil {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.LastScheduleTime = &metav1.Time{
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

// SetupWithManager sets up the controller with the Manager.
func (r *PhysicalBackupReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.PhysicalBackup{}).
		Owns(&batchv1.Job{}).
		Owns(&volumesnapshotv1.VolumeSnapshot{})
	if err := r.indexJobs(ctx, mgr); err != nil {
		return fmt.Errorf("error indexing PhysicalBackup Jobs: %v", err)
	}
	return builder.Complete(r)
}

func getObjectName(obj client.Object, now time.Time) string {
	return fmt.Sprintf("%s-%s", obj.GetName(), mdbtime.Format(now))
}

func parseObjectTime(obj client.Object) (time.Time, error) {
	parts := strings.Split(obj.GetName(), "-")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid object name \"%s\"", obj.GetName())
	}
	return mdbtime.Parse(parts[1])
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
