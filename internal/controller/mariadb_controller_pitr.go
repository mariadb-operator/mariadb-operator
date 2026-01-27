package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/client"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MariaDBReconciler) reconcilePITR(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !shouldReconcilePITR(mdb) {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx).WithName("pitr")

	startGtid, err := r.getStartGtid(ctx, mdb, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting start GTID: %v", err)
	}
	if !mdb.IsReplayingBinlogs() || mdb.ReplayBinlogsError() != nil {
		if result, err := r.reconcileReplayBinlogsError(ctx, mdb, startGtid, logger); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if !mdb.IsReplayingBinlogs() {
		logger.Info("Replaying binlogs")
		if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetReplayingBinlogs(status)
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}
	}

	// TODO: disable gtid_strict_mode if needed

	if err := r.reconcilePITRStagingPVC(ctx, mdb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAndWaitForPITRJob(ctx, mdb, startGtid, logger); !result.IsZero() || err != nil {
		return result, err
	}

	// TODO: restore gtid_strict_mode if needed

	logger.Info("Binlogs replayed")
	if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplayedBinlogs(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) getStartGtid(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (*replication.Gtid, error) {
	var rawGtid string

	if mdb.Spec.BootstrapFrom != nil && mdb.Spec.BootstrapFrom.VolumeSnapshotRef != nil {
		key := types.NamespacedName{
			Name:      mdb.Spec.BootstrapFrom.VolumeSnapshotRef.Name,
			Namespace: mdb.Namespace,
		}
		var snapshot volumesnapshotv1.VolumeSnapshot

		if err := r.Get(ctx, key, &snapshot); err != nil {
			return nil, fmt.Errorf("error getting VolumeSnapshot: %v", err)
		}
		snapGtid, ok := snapshot.Annotations[metadata.GtidAnnotation]
		if !ok {
			return nil, fmt.Errorf("annotation %s not found in VolumeSnapshot %s", metadata.GtidAnnotation, snapshot.Name)
		}
		logger.Info("Got GTID from VolumeSnapshot", "gtid", snapGtid, "snapshot", snapshot.Name)
		rawGtid = snapGtid
	} else {
		if mdb.Status.CurrentPrimaryPodIndex == nil {
			return nil, errors.New("status.currentPrimaryPodIndex must be set")
		}
		agentClient, err := agentclient.NewClientWithMariaDB(mdb, *mdb.Status.CurrentPrimaryPodIndex)
		if err != nil {
			return nil, fmt.Errorf("error getting agent client: %v", err)
		}

		// TODO: handle galera, as the agent will not have this endpoint available
		agentGtid, err := agentClient.Replication.GetGtid(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting GTID from agent: %v", err)
		}
		logger.Info("Got GTID from agent", "gtid", agentGtid)
		rawGtid = agentGtid
	}
	if rawGtid == "" {
		return nil, errors.New("GTID not found")
	}

	client, err := sql.NewClientWithMariaDB(ctx, mdb, r.RefResolver)
	if err != nil {
		return nil, fmt.Errorf("error getting SQL client: %v", err)
	}
	defer client.Close()

	domainId, err := client.GtidDomainId(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting gtid_domain_id: %v", err)
	}
	gtid, err := replication.ParseGtidWithDomainId(rawGtid, *domainId, logger.WithName("gtid"))
	if err != nil {
		return nil, fmt.Errorf("error parsing GTID %s: %v", rawGtid, err)
	}
	return gtid, nil
}

func (r *MariaDBReconciler) reconcileReplayBinlogsError(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, startGtid *replication.Gtid,
	logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Validating binlog path")
	if err := r.validateBinlogPath(ctx, mariadb, startGtid); err != nil {
		errMsg := fmt.Sprintf("Invalid binary log path: %v", err)
		r.Recorder.Event(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonMariaDBInvalidBinlogPath, errMsg)

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetReplayBinlogsError(status, errMsg)
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}

		logger.Error(err, errMsg)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) validateBinlogPath(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, startGtid *replication.Gtid) error {
	return nil
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

func (r *MariaDBReconciler) reconcileAndWaitForPITRJob(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, startGtid *replication.Gtid,
	logger logr.Logger) (ctrl.Result, error) {
	key := mdb.PITRJobKey()
	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating PointInTimeRecovery job", "name", key.Name)
			if err := r.createPITRJob(ctx, mdb, startGtid); err != nil {
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

func (r *MariaDBReconciler) createPITRJob(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, startGtid *replication.Gtid) error {
	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mdb.Spec.PointInTimeRecoveryRef, mdb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	pitrJob, err := r.Builder.BuildPITRJob(
		mdb.PITRJobKey(),
		pitr,
		mdb,
		builder.WithStartGtid(startGtid),
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
