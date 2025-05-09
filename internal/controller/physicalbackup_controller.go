package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	jobpkg "github.com/mariadb-operator/mariadb-operator/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const jobMetaCtrlFieldPath = ".metadata.controller"

// PhysicalBackupReconciler reconciles a PhysicalBackup object
type PhysicalBackupReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Builder           *builder.Builder
	RefResolver       *refresolver.RefResolver
	ConditionComplete *condition.Complete
	RBACReconciler    *rbac.RBACReconciler
	BatchReconciler   *batch.BatchReconciler
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

	jobList, err := r.listJobs(ctx, &backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error listing Jobs: %v", err)
	}

	if err := r.reconcileStatus(ctx, &backup, jobList); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling status: %v", err)
	}

	mariadb, err := r.RefResolver.MariaDB(ctx, &backup.Spec.MariaDBRef, backup.Namespace)
	if err != nil {
		var mariaDbErr *multierror.Error
		mariaDbErr = multierror.Append(mariaDbErr, err)

		err = r.patchStatus(ctx, &backup, r.ConditionComplete.PatcherRefResolver(err, mariadb))
		mariaDbErr = multierror.Append(mariaDbErr, err)

		return ctrl.Result{}, fmt.Errorf("error getting MariaDB: %v", mariaDbErr)
	}
	if backup.Spec.MariaDBRef.WaitForIt && !mariadb.IsReady() {
		if err := r.patchStatus(ctx, &backup, r.ConditionComplete.PatcherFailed("MariaDB not ready")); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Backup: %v", err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if err := r.setDefaults(ctx, &backup, mariadb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error defaulting PhysicalBackup: %v", err)
	}

	if err := r.reconcileServiceAccount(ctx, &backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling ServiceAccount: %v", err)
	}

	if err := r.reconcileStorage(ctx, &backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("error reconciling storage: %v", err)
	}

	return r.reconcileJobs(ctx, &backup, jobList, mariadb)
}

func (r *PhysicalBackupReconciler) listJobs(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup) (*batchv1.JobList, error) {
	var jobList batchv1.JobList
	if err := r.List(
		ctx,
		&jobList,
		client.InNamespace(backup.Namespace),
		client.MatchingFields{jobMetaCtrlFieldPath: backup.Name},
	); err != nil {
		return nil, err
	}
	return &jobList, nil
}

func (r *PhysicalBackupReconciler) indexJobs(ctx context.Context, mgr manager.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		ctx,
		&batchv1.Job{},
		jobMetaCtrlFieldPath,
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

func (r *PhysicalBackupReconciler) reconcileStatus(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	jobList *batchv1.JobList) error {
	logger := log.FromContext(ctx).WithName("status").V(1)

	numRunning := 0
	numComplete := 0
	for _, job := range jobList.Items {
		if jobpkg.IsJobFailed(&job) {
			if err := r.patchStatus(ctx, backup, func(c condition.Conditioner) {
				backup.Status.SetCondition(metav1.Condition{
					Type:    mariadbv1alpha1.ConditionTypeComplete,
					Status:  metav1.ConditionTrue,
					Reason:  mariadbv1alpha1.ConditionReasonJobFailed,
					Message: "Failed",
				})
			}); err != nil {
				logger.Info("error patching status", "err", err)
			}
		} else if jobpkg.IsJobRunning(&job) {
			numRunning++
		} else if jobpkg.IsJobComplete(&job) {
			numComplete++
		}
	}

	if numComplete == len(jobList.Items) {
		if err := r.patchStatus(ctx, backup, func(c condition.Conditioner) {
			backup.Status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionTrue,
				Reason:  mariadbv1alpha1.ConditionReasonJobComplete,
				Message: "Success",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	} else if numRunning > 0 {
		if err := r.patchStatus(ctx, backup, func(c condition.Conditioner) {
			backup.Status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonJobRunning,
				Message: "Running",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	} else {
		if err := r.patchStatus(ctx, backup, func(c condition.Conditioner) {
			backup.Status.SetCondition(metav1.Condition{
				Type:    mariadbv1alpha1.ConditionTypeComplete,
				Status:  metav1.ConditionFalse,
				Reason:  mariadbv1alpha1.ConditionReasonJobNotComplete,
				Message: "Not complete",
			})
		}); err != nil {
			logger.Info("error patching status", "err", err)
		}
	}
	return nil
}

func (r *PhysicalBackupReconciler) setDefaults(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) error {
	return r.patch(ctx, backup, func(b *mariadbv1alpha1.PhysicalBackup) {
		backup.SetDefaults(mariadb)
	})
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
		if err := r.createPVC(ctx, pvc); err != nil {
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
		if err := r.createPVC(ctx, pvc); err != nil {
			return fmt.Errorf("error creating Backup staging PVC: %v", err)
		}
	}

	return nil
}

func (r *PhysicalBackupReconciler) createPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	key := client.ObjectKeyFromObject(pvc)
	var existingPvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, key, &existingPvc)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error creating PersistentVolumeClaim: %v", err)
	}
	return r.Create(ctx, pvc)
}

func (r *PhysicalBackupReconciler) reconcileJobs(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	jobList *batchv1.JobList, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if len(jobList.Items) == 0 {
		return r.createJob(ctx, backup, mariadb)
	}
	return ctrl.Result{}, nil
}

func (r *PhysicalBackupReconciler) createJob(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
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
		Name:      fmt.Sprintf("%s-%d", backup.Name, time.Now().UnixMilli()),
		Namespace: mariadb.Namespace,
	}
	backupFileName, err := r.getBackupFilename(backup)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting backup file name: %v", err)
	}

	job, err := r.Builder.BuildPhysicalBackupJob(backupKey, backup, mariadb, &primaryPod, backupFileName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error building Job: %v", err)
	}
	return ctrl.Result{}, r.Create(ctx, job)
}

func (r *PhysicalBackupReconciler) getBackupFilename(backup *mariadbv1alpha1.PhysicalBackup) (string, error) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	backupFile := fmt.Sprintf("physicalbackup-%s.xb", timestamp)

	if backup.Spec.Compression != "" && backup.Spec.Compression != mariadbv1alpha1.CompressNone {
		ext, err := backup.Spec.Compression.Extension()
		if err != nil {
			return "", fmt.Errorf("error getting compression algorithm extension: %v", err)
		}
		backupFile = fmt.Sprintf("%s.%s", backupFile, ext)
	}
	return backupFile, nil
}

func (r *PhysicalBackupReconciler) patch(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	patcher func(*mariadbv1alpha1.PhysicalBackup)) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(backup)
	return r.Patch(ctx, backup, patch)
}

func (r *PhysicalBackupReconciler) patchStatus(ctx context.Context, backup *mariadbv1alpha1.PhysicalBackup,
	patcher condition.Patcher) error {
	patch := client.MergeFrom(backup.DeepCopy())
	patcher(&backup.Status)
	return r.Client.Status().Patch(ctx, backup, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PhysicalBackupReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mariadbv1alpha1.PhysicalBackup{}).
		Owns(&batchv1.Job{})

	if err := r.indexJobs(ctx, mgr); err != nil {
		return fmt.Errorf("error indexing PhysicalBackup Jobs: %v", err)
	}

	return builder.Complete(r)
}
