package controller

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	jobpkg "github.com/mariadb-operator/mariadb-operator/pkg/job"
	mdbtime "github.com/mariadb-operator/mariadb-operator/pkg/time"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
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

func (r *PhysicalBackupReconciler) reconcileJobs(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	jobList, err := r.listJobs(ctx, backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Jobs: %v", err)
	}
	if err := r.reconcileJobStatus(ctx, backup, jobList); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling status: %v", err)
	}
	if err := r.cleanupJobs(ctx, backup, jobList); err != nil {
		return ctrl.Result{}, fmt.Errorf("error cleaning up Jobs: %v", err)
	}

	if err := r.reconcileServiceAccount(ctx, backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling ServiceAccount: %v", err)
	}
	if err := r.reconcileStorage(ctx, backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling storage: %v", err)
	}

	return r.reconcileTemplate(ctx, backup, len(jobList.Items), func(now time.Time, cronSchedule cron.Schedule) (ctrl.Result, error) {
		return r.createJob(ctx, backup, mariadb, jobList, now, cronSchedule)
	})
}

func (r *PhysicalBackupReconciler) listJobs(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup) (*batchv1.JobList, error) {
	var jobList batchv1.JobList
	if err := r.List(
		ctx,
		&jobList,
		client.InNamespace(backup.Namespace),
		client.MatchingFields{metaCtrlFieldPath: backup.Name},
	); err != nil {
		return nil, err
	}
	return &jobList, nil
}

func (r *PhysicalBackupReconciler) indexJobs(ctx context.Context, mgr manager.Manager) error {
	log.FromContext(ctx).
		WithName("indexer").
		WithValues(
			"kind", "Job",
			"field", metaCtrlFieldPath,
		).
		Info("Watching field")
	return mgr.GetFieldIndexer().IndexField(
		ctx,
		&batchv1.Job{},
		metaCtrlFieldPath,
		func(o client.Object) []string {
			job, ok := o.(*batchv1.Job)
			if !ok {
				return nil
			}
			owner := metav1.GetControllerOf(job)
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

func (r *PhysicalBackupReconciler) reconcileJobStatus(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	jobList *batchv1.JobList) error {
	logger := log.FromContext(ctx).WithName("status").V(1)
	schedule := ptr.Deref(backup.Spec.Schedule, mariadbv1alpha1.PhysicalBackupSchedule{})

	if schedule.Suspend {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonJobSuspended,
				Message: "Suspended",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
		return nil
	}

	numRunning := 0
	numComplete := 0
	for _, job := range jobList.Items {
		if jobpkg.IsJobFailed(&job) {
			if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
				status.SetCondition(metav1.Condition{
					Type:    mariadbv1alpha1.ConditionTypeComplete,
					Status:  metav1.ConditionTrue,
					Reason:  mariadbv1alpha1.ConditionReasonJobFailed,
					Message: "Failed",
				})
			}); err != nil {
				logger.Info("error patching status", "err", err)
			}
			return nil
		} else if jobpkg.IsJobRunning(&job) {
			numRunning++
		} else if jobpkg.IsJobComplete(&job) {
			numComplete++
		}
	}

	if len(jobList.Items) > 0 && numComplete == len(jobList.Items) {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonJobComplete,
				Message: "Success",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	} else if len(jobList.Items) > 0 && numRunning > 0 {
		if err := r.patchStatus(ctx, backup, func(status *mariadbv1alpha1.PhysicalBackupStatus) {
			status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonJobRunning,
				Message: "Running",
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
				Reason:  mariadbv1alpha1.ConditionReasonJobNotComplete,
				Message: message,
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	}
	return nil
}

func (r *PhysicalBackupReconciler) cleanupJobs(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	jobList *batchv1.JobList) error {
	if backup.Spec.Schedule == nil {
		return nil
	}

	var completeJobs []*batchv1.Job
	for _, job := range jobList.Items {
		if jobpkg.IsJobComplete(&job) {
			completeJobs = append(completeJobs, &job)
		}
	}
	maxHistory := int(ptr.Deref(backup.Spec.SuccessfulJobsHistoryLimit, 5))
	if len(completeJobs) <= maxHistory {
		return nil
	}

	if err := sortByObjectTime(completeJobs); err != nil {
		return err
	}
	logger := log.FromContext(ctx).WithName("job")

	for i := maxHistory; i < len(completeJobs); i++ {
		job := completeJobs[i]

		err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting Job \"%s\": %v", job.Name, err)
		}
		logger.V(1).Info("Deleted old Job", "job", job.Name, "physicalbackup", backup.Name)
	}

	return nil
}

func (r *PhysicalBackupReconciler) reconcileServiceAccount(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup) error {
	key := backup.Spec.ServiceAccountKey(backup.ObjectMeta)
	_, err := r.RBACReconciler.ReconcileServiceAccount(ctx, key, backup, backup.Spec.InheritMetadata)
	return err
}

func (r *PhysicalBackupReconciler) reconcileStorage(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup) error {
	if backup.Spec.Storage.PersistentVolumeClaim != nil {
		pvc, err := r.Builder.BuildBackupStoragePVC(
			backup.StoragePVCKey(),
			backup.Spec.Storage.PersistentVolumeClaim,
			backup.Spec.InheritMetadata,
		)
		if err != nil {
			return fmt.Errorf("error building Backup storage PVC: %v", err)
		}
		if err := r.PVCReconciler.Reconcile(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
			return fmt.Errorf("error creating Backup storage PVC: %v", err)
		}
	}

	stagingStorage := ptr.Deref(backup.Spec.StagingStorage, mariadbv1alpha1.BackupStagingStorage{})
	if stagingStorage.PersistentVolumeClaim != nil {
		pvc, err := r.Builder.BuildBackupStagingPVC(
			backup.StagingPVCKey(),
			stagingStorage.PersistentVolumeClaim,
			backup.Spec.InheritMetadata,
			backup,
		)
		if err != nil {
			return fmt.Errorf("error building Backup staging PVC: %v", err)
		}
		if err := r.PVCReconciler.Reconcile(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
			return fmt.Errorf("error creating Backup staging PVC: %v", err)
		}
	}

	return nil
}

func (r *PhysicalBackupReconciler) createJob(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup, mariadb *mariadbv1alpha1.MariaDB,
	jobList *batchv1.JobList, now time.Time, schedule cron.Schedule) (ctrl.Result, error) {
	if result, err := r.waitForRunningJobs(ctx, jobList); !result.IsZero() || err != nil {
		return result, err
	}
	if mariadb.Status.CurrentPrimary == nil {
		log.FromContext(ctx).V(1).Info("Current primary not set. Requeuing...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	primaryPodKey := types.NamespacedName{
		Name:      *mariadb.Status.CurrentPrimary,
		Namespace: mariadb.Namespace,
	}
	var primaryPod corev1.Pod
	if err := r.Get(ctx, primaryPodKey, &primaryPod); err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting primary Pod: %v", err)
	}

	backupKey := types.NamespacedName{
		Name:      getObjectName(backup, now),
		Namespace: mariadb.Namespace,
	}
	backupFileName, err := getBackupFileName(backup, now)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting backup file name: %v", err)
	}

	job, err := r.Builder.BuildPhysicalBackupJob(backupKey, backup, mariadb, &primaryPod, backupFileName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building Job: %v", err)
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
		mariadbv1alpha1.ReasonJobScheduled,
		"Job %s scheduled",
		job.Name,
	)

	return ctrl.Result{}, r.Create(ctx, job)
}

func (r *PhysicalBackupReconciler) waitForRunningJobs(ctx context.Context, jobList *batchv1.JobList) (ctrl.Result, error) {
	for _, job := range jobList.Items {
		if !jobpkg.IsJobComplete(&job) {
			log.FromContext(ctx).Info("PhysicalBackup Job is not complete. Requeuing...", "job", job.Name)
			return ctrl.Result{Requeue: true}, nil
		}
	}
	return ctrl.Result{}, nil
}

func getBackupFileName(backup *mariadbv1alpha1.PhysicalBackup, now time.Time) (string, error) {
	backupFile := fmt.Sprintf("physicalbackup-%s.xb", mdbtime.Format(now))

	if backup.Spec.Compression != "" && backup.Spec.Compression != mariadbv1alpha1.CompressNone {
		ext, err := backup.Spec.Compression.Extension()
		if err != nil {
			return "", fmt.Errorf("error getting compression algorithm extension: %v", err)
		}
		backupFile = fmt.Sprintf("%s.%s", backupFile, ext)
	}
	return backupFile, nil
}
