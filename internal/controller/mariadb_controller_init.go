package controller

import (
	"context"
	"fmt"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	jobpkg "github.com/mariadb-operator/mariadb-operator/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/pkg/pvc"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcileInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mariadb.Spec.BootstrapFrom != nil && mariadb.Spec.BootstrapFrom.BackupType == mariadbv1alpha1.BackupTypePhysical {
		return r.reconcilePhysicalBackupInit(ctx, mariadb)
	} else if mariadb.IsGaleraEnabled() {
		if result, err := r.GaleraReconciler.ReconcileInit(ctx, mariadb); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcilePhysicalBackupInit(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if mariadb.IsInitialized() {
		return ctrl.Result{}, nil
	}

	if !mariadb.IsInitializing() || mariadb.InitError() != nil {
		pvcs, err := pvc.ListStoragePVCs(ctx, r.Client, mariadb)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error listing PVCs: %v", err)
		}
		if len(pvcs) > 0 {
			r.Recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBInitError,
				"Unable to init MariaDB: storage PVCs already exist")

			if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
				condition.SetInitError(status, "storage PVCs already exist")
				return nil
			}); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
			}

			log.FromContext(ctx).Info("Unable to init MariaDB: storage PVCs already exist. Requeuing...")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetInitializing(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}

	if err := r.reconcilePVCs(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAndWaitForInitJobs(ctx, mariadb); !result.IsZero() || err != nil {
		return result, err
	}
	if err := r.cleanupInitJobs(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.cleanupStagingPVC(ctx, mariadb); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetInitialized(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcilePVCs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		key := mariadb.PVCKey(builder.StorageVolume, i)
		pvc, err := r.Builder.BuildStoragePVC(key, mariadb.Spec.Storage.VolumeClaimTemplate, mariadb)
		if err != nil {
			return err
		}
		if err := r.PVCReconciler.Reconcile(ctx, key, pvc); err != nil {
			return err
		}
	}

	if mariadb.Spec.BootstrapFrom.ShouldProvisionPhysicalBackupStagingPVC() {
		key := mariadb.PhysicalBackupStagingPVCKey()
		pvc, err := r.Builder.BuildBackupStagingPVC(
			key,
			mariadb.Spec.BootstrapFrom.StagingStorage.PersistentVolumeClaim,
			mariadb.Spec.InheritMetadata,
			mariadb,
		)
		if err != nil {
			return err
		}
		if err := r.PVCReconciler.Reconcile(ctx, key, pvc); err != nil {
			return err
		}
	}

	return nil
}

func (r *MariaDBReconciler) reconcileAndWaitForInitJobs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	return r.forEachPhysicalBackupKey(mariadb, func(key types.NamespacedName, podIndex int) (ctrl.Result, error) {
		var job batchv1.Job
		if err := r.Get(ctx, key, &job); err != nil {
			if apierrors.IsNotFound(err) {
				if err := r.createInitJob(ctx, mariadb, key, podIndex); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}
		}

		if !jobpkg.IsJobComplete(&job) {
			log.FromContext(ctx).V(1).Info("PhysicalBackup init job not completed. Requeuing")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		return ctrl.Result{}, nil
	})
}

func (r *MariaDBReconciler) createInitJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	key types.NamespacedName, podIndex int) error {
	job, err := r.Builder.BuildPhysicalBackupInitJob(key, mariadb, &podIndex)
	if err != nil {
		return fmt.Errorf("error building PhysicalBackup init Job: %v", err)
	}
	return r.Create(ctx, job)
}

func (r *MariaDBReconciler) cleanupInitJobs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	_, err := r.forEachPhysicalBackupKey(mariadb, func(key types.NamespacedName, podIndex int) (ctrl.Result, error) {
		var job batchv1.Job
		if err := r.Get(ctx, key, &job); err == nil {
			if err := r.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)}); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}
		return ctrl.Result{}, nil
	})
	return err
}

func (r *MariaDBReconciler) cleanupStagingPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !mariadb.Spec.BootstrapFrom.ShouldProvisionPhysicalBackupStagingPVC() {
		return nil
	}
	key := mariadb.PhysicalBackupStagingPVCKey()
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, key, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, &pvc)
}

func (r *MariaDBReconciler) forEachPhysicalBackupKey(mariadb *mariadbv1alpha1.MariaDB,
	fn func(key types.NamespacedName, podIndex int) (ctrl.Result, error)) (ctrl.Result, error) {
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if result, err := fn(mariadb.PhysicalBackupInitJobKey(i), i); !result.IsZero() || err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}
