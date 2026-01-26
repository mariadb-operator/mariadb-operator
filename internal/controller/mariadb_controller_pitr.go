package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/job"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcilePITR(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcilePITR(mdb) {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("pitr")

	if !mdb.IsReplayingBinlogs() {
		logger.Info("Replaying binlogs")
		if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetReplayingBinlogs(status)
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}
	}

	if err := r.reconcilePITRStagingPVC(ctx, mdb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAndWaitForPITRJob(ctx, mdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	logger.Info("Binlogs replayed")
	if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplayedBinlogs(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) reconcilePITRStagingPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if shouldProvisionPITRStagingPVC(mariadb) {
		key := mariadb.PITRStagingPVCKey()
		pvc, err := r.Builder.BuildStagingPVC(
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

func (r *MariaDBReconciler) reconcileAndWaitForPITRJob(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (ctrl.Result, error) {
	key := mdb.PITRJobKey()
	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating PointInTimeRecovery job", "name", key.Name)
			if err := r.createPITRJob(ctx, mdb); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		} else {
			return ctrl.Result{}, fmt.Errorf("error getting PointInTimeRecovery Job: %v", err)
		}
	}
	if !jobpkg.IsJobComplete(&job) {
		logger.V(1).Info("PointInTimeRecovery job not completed. Requeuing...")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) createPITRJob(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mdb.Spec.PointInTimeRecoveryRef, mdb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	pitrJob, err := r.Builder.BuildPITRJob(
		mdb.PITRJobKey(),
		pitr,
		mdb,
		builder.WithBootstrapFrom(mdb.Spec.BootstrapFrom),
	)
	if err != nil {
		return fmt.Errorf("error building PointInTimeRecovery Job: %v", err)
	}
	return r.Create(ctx, pitrJob)
}

func shouldReconcilePITR(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsInitializing() || mdb.IsUpdating() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() ||
		mdb.IsScalingOut() || mdb.IsRecoveringReplicas() || mdb.HasGaleraNotReadyCondition() ||
		mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() {
		return false
	}
	if mdb.HasReplayedBinlogs() {
		return false
	}
	return mdb.Spec.BootstrapFrom != nil && mdb.Spec.BootstrapFrom.PointInTimeRecoveryRef != nil
}

func shouldProvisionPITRStagingPVC(mariadb *mariadbv1alpha1.MariaDB) bool {
	b := mariadb.Spec.BootstrapFrom
	if b == nil {
		return false
	}
	return b.PointInTimeRecoveryRef != nil && b.StagingStorage != nil && b.StagingStorage.PersistentVolumeClaim != nil
}
