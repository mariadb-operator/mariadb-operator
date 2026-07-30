package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	agentclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/agent/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/azure"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/binlog"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	replicationctrl "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/gtid"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var errSkipBinlogReplay = errors.New("skip binlog replay")

func (r *MariaDBReconciler) reconcilePITR(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("pitr")
	shouldReconcile, err := r.shouldReconcilePITR(ctx, mdb, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error determining whether PITR should be reconciled: %v", err)
	}
	if !shouldReconcile {
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues(
		"target-time", mdb.Spec.BootstrapFrom.TargetRecoveryTimeOrDefault().Format(time.RFC3339),
	)
	if !mdb.IsReplayingBinlogs() || mdb.ReplayBinlogsError() != nil {
		result, err := r.reconcileReplayBinlogsError(ctx, mdb, logger)

		if errors.Is(err, errSkipBinlogReplay) {
			if !mdb.HasSkippedBinlogReplay() {
				if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
					condition.SetReplayBinlogsSkipped(status)
					return nil
				}); err != nil {
					return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
				}
			}
			return ctrl.Result{}, nil
		}
		if !result.IsZero() || err != nil {
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

	if err := r.prepareBinlogReplay(ctx, mdb, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error preparing binlog replay: %v", err)
	}

	if err := r.reconcilePITRStagingPVC(ctx, mdb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAndWaitForPITRJob(ctx, mdb, logger); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.finishBinlogReplay(ctx, mdb, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error finishing binlog replay: %v", err)
	}

	logger.Info("Binlogs replayed")
	if err := r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		condition.SetReplayedBinlogs(status)
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
	}
	if err := r.cleanupPITRJob(ctx, mdb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error cleaning up PITR job: %v", err)
	}
	if err := r.cleanupPITRStagingPVC(ctx, mdb); err != nil {
		return ctrl.Result{}, fmt.Errorf("error cleaning up PITR stating PVC: %v", err)
	}
	return ctrl.Result{}, nil
}

// prepareBinlogReplay temporarily relaxes server settings for binlog replay:
// - Galera: disables wsrep_gtid_mode on the archiver Pod so server_id can be overridden. wsrep_on remains enabled, so replayed transactions replicate normally.
// - Replication: disables gtid_strict_mode on the primary.
// - Standalone: no changes required.
func (r *MariaDBReconciler) prepareBinlogReplay(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	if mdb.IsGaleraEnabled() {
		return r.pauseWsrepGtidMode(ctx, mdb, logger)
	}
	if mdb.IsReplicationEnabled() {
		sqlClient, err := sql.NewClientWithMariaDB(ctx, mdb, r.RefResolver)
		if err != nil {
			return fmt.Errorf("error getting SQL client: %v", err)
		}
		defer sqlClient.Close()
		return replicationctrl.PauseGtidStrictMode(ctx, mdb, sqlClient, r.Client, logger)
	}
	return nil
}

// finishBinlogReplay reverts the changes made by prepareBinlogReplay once the replay Job has completed.
func (r *MariaDBReconciler) finishBinlogReplay(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	if mdb.IsGaleraEnabled() {
		return r.resumeWsrepGtidMode(ctx, mdb, logger)
	}
	if mdb.IsReplicationEnabled() {
		sqlClient, err := sql.NewClientWithMariaDB(ctx, mdb, r.RefResolver)
		if err != nil {
			return fmt.Errorf("error getting SQL client: %v", err)
		}
		defer sqlClient.Close()
		return replicationctrl.ResumeGtidStrictMode(ctx, mdb, sqlClient, r.Client, logger)
	}
	return nil
}

// binlogArchiverPodIndex returns the Pod index that archives binary logs and into which they are replayed. It is resolved from the PointInTimeRecovery
// referred by the MariaDB itself (spec.pointInTimeRecoveryRef), which is the archive that the binlog timeline continues into, and not from the one being
// restored from (bootstrapFrom).
func (r *MariaDBReconciler) binlogArchiverPodIndex(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (int, error) {
	if !mdb.IsPointInTimeRecoveryEnabled() {
		return 0, nil
	}
	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mdb.Spec.PointInTimeRecoveryRef, mdb.Namespace)
	if err != nil {
		return 0, fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	podIndex := pitr.PodArchiverIndex()

	if podIndex >= int(mdb.Spec.Replicas) {
		return 0, fmt.Errorf(
			"invalid spec.podArchiverIndex %d in PointInTimeRecovery \"%s\": it must be lower than spec.replicas (%d) in MariaDB",
			podIndex, pitr.Name, mdb.Spec.Replicas,
		)
	}
	return podIndex, nil
}

func (r *MariaDBReconciler) pauseWsrepGtidMode(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	pitrStatus := ptr.Deref(mdb.Status.PointInTimeRecovery, mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{})
	if ptr.Deref(pitrStatus.WsrepGtidModePaused, false) {
		return nil
	}

	podIndex, err := r.binlogArchiverPodIndex(ctx, mdb)
	if err != nil {
		return fmt.Errorf("error getting binlog archiver Pod index: %v", err)
	}
	sqlClient, err := sql.NewInternalClientWithPodIndex(ctx, mdb, r.RefResolver, podIndex)
	if err != nil {
		return fmt.Errorf("error getting SQL client for Pod index %d: %v", podIndex, err)
	}
	defer sqlClient.Close()

	wsrepGtidMode, err := sqlClient.WsrepGtidMode(ctx)
	if err != nil {
		return fmt.Errorf("error getting wsrep_gtid_mode: %v", err)
	}
	if wsrepGtidMode {
		logger.Info("Temporarily disabling wsrep_gtid_mode to replay binlogs", "pod-index", podIndex)
		if err := sqlClient.DisableWsrepGtidMode(ctx); err != nil {
			return fmt.Errorf("error disabling wsrep_gtid_mode: %v", err)
		}
	}
	return r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if status.PointInTimeRecovery == nil {
			status.PointInTimeRecovery = &mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{}
		}
		status.PointInTimeRecovery.WsrepGtidModePaused = ptr.To(true)
		return nil
	})
}

// resumeWsrepGtidMode re-enables wsrep_gtid_mode on the archiver Pod once the replay has finished.
func (r *MariaDBReconciler) resumeWsrepGtidMode(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	pitrStatus := ptr.Deref(mdb.Status.PointInTimeRecovery, mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{})
	if !ptr.Deref(pitrStatus.WsrepGtidModePaused, false) {
		return nil
	}

	podIndex, err := r.binlogArchiverPodIndex(ctx, mdb)
	if err != nil {
		return fmt.Errorf("error getting binlog archiver Pod index: %v", err)
	}
	sqlClient, err := sql.NewInternalClientWithPodIndex(ctx, mdb, r.RefResolver, podIndex)
	if err != nil {
		return fmt.Errorf("error getting SQL client for Pod index %d: %v", podIndex, err)
	}
	defer sqlClient.Close()

	logger.Info("Enabling back wsrep_gtid_mode after binlog replay", "pod-index", podIndex)
	if err := sqlClient.EnableWsrepGtidMode(ctx); err != nil {
		return fmt.Errorf("error enabling wsrep_gtid_mode: %v", err)
	}
	return r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if status.PointInTimeRecovery != nil {
			status.PointInTimeRecovery.WsrepGtidModePaused = nil
		}
		return nil
	})
}

func (r *MariaDBReconciler) getStartGtid(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	logger logr.Logger) (*gtid.Gtid, error) {
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
		logger.V(1).Info("Got GTID from VolumeSnapshot", "gtid", snapGtid, "snapshot", snapshot.Name)
		rawGtid = snapGtid
	} else {
		if mdb.Status.CurrentPrimaryPodIndex == nil {
			return nil, errors.New("status.currentPrimaryPodIndex must be set")
		}
		agentClient, err := agentclient.NewClientWithMariaDB(ctx, mdb, r.Environment, r.RefResolver, *mdb.Status.CurrentPrimaryPodIndex)
		if err != nil {
			return nil, fmt.Errorf("error getting agent client: %v", err)
		}

		agentGtid, err := agentClient.Gtid.GetGtid(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting GTID from agent: %v", err)
		}
		logger.V(1).Info("Got GTID from agent", "gtid", agentGtid)
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
	gtid, err := gtid.ParseGtidWithDomainId(rawGtid, *domainId, logger.WithName("gtid"))
	if err != nil {
		return nil, fmt.Errorf("error parsing GTID %s: %v", rawGtid, err)
	}
	return gtid, nil
}

func (r *MariaDBReconciler) reconcileReplayBinlogsError(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	binlogLogger logr.Logger) (ctrl.Result, error) {
	startGtid, err := r.getStartGtid(ctx, mariadb, binlogLogger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting start GTID: %v", err)
	}
	logger := binlogLogger.WithValues("start-gtid", startGtid)

	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mariadb.Spec.BootstrapFrom.PointInTimeRecoveryRef, mariadb.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting PointInTimeRecoveryRef: %v", err)
	}
	storageClient, err := r.getStorageClient(ctx, pitr)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting S3 client: %v", err)
	}
	if !storageClient.IsAuthenticated(ctx) {
		logger.Info("Object storage credentials not found. Skipping binlog timeline validation...", "err", err)
		return ctrl.Result{}, nil
	}

	logger.Info("Validating binlog timeline")
	if err := r.validateBinlogTimeline(ctx, mariadb, startGtid, pitr.Spec.StrictMode, storageClient, logger); err != nil {
		if errors.Is(err, binlog.ErrNoBinlogs) && !pitr.Spec.StrictMode {
			logger.Info("No binlogs available and strict mode is disabled. Skipping binlog replay...", "err", err)
			return ctrl.Result{}, errSkipBinlogReplay
		}
		errMsg := fmt.Sprintf("Invalid binary log timeline: %v", err)
		logger.Error(err, errMsg)
		r.Recorder.Eventf(mariadb, pitr, corev1.EventTypeWarning, mariadbv1alpha1.ReasonBinlogTimelineInvalid,
			mariadbv1alpha1.ActionReconciling, errMsg)

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
			condition.SetReplayBinlogsError(status, errMsg)
			return nil
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching MariaDB status: %v", err)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MariaDBReconciler) validateBinlogTimeline(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, startGtid *gtid.Gtid,
	strictMode bool, storageClient interfaces.BlobStorage, logger logr.Logger) error {
	indexReader, err := storageClient.GetObjectWithOptions(ctx, binlog.BinlogIndexName)
	if err != nil {
		return fmt.Errorf("error getting binlog index: %v", err)
	}
	defer indexReader.Close()
	indexBytes, err := io.ReadAll(indexReader)
	if err != nil {
		return fmt.Errorf("error reading binlog index: %v", err)
	}
	var index binlog.BinlogIndex
	if err := yaml.Unmarshal(indexBytes, &index); err != nil {
		return fmt.Errorf("error unmarshalling binlog index: %v", err)
	}

	targetTime := mdb.Spec.BootstrapFrom.TargetRecoveryTimeOrDefault()
	binlogMetas, err := index.BuildTimeline(startGtid, targetTime, strictMode, logger)
	if err != nil {
		return fmt.Errorf(
			"error getting binlog timeline between GTID %s and target time %s: %w",
			startGtid.String(),
			targetTime.Format(time.RFC3339),
			err,
		)
	}
	binlogPath := make([]string, len(binlogMetas))
	for i, meta := range binlogMetas {
		binlogPath[i] = meta.ObjectStoragePath()
	}
	logger.Info("Got binlog timeline", "timeline", binlogPath)

	return nil
}

func (r *MariaDBReconciler) reconcilePITRStagingPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if shouldProvisionPITRStagingPVC(mariadb) {
		key := mariadb.BootstrapFromStagingPVCKey()
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
			startGtid, err := r.getStartGtid(ctx, mdb, logger)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error getting start GTID: %v", err)
			}
			logger.Info("Creating PointInTimeRecovery job", "name", key.Name, "start-gtid", startGtid)
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

func (r *MariaDBReconciler) createPITRJob(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, startGtid *gtid.Gtid) error {
	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mdb.Spec.BootstrapFrom.PointInTimeRecoveryRef, mdb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	podArchiverIndex, err := r.binlogArchiverPodIndex(ctx, mdb)
	if err != nil {
		return fmt.Errorf("error getting binlog archiver Pod index: %v", err)
	}
	pitrJob, err := r.Builder.BuildPITRJob(
		mdb.PITRJobKey(),
		pitr,
		mdb,
		builder.WithStartGtid(startGtid),
		builder.WithBootstrapFrom(mdb.Spec.BootstrapFrom),
		builder.WithPodArchiverIndex(podArchiverIndex),
	)
	if err != nil {
		return fmt.Errorf("error building PointInTimeRecovery Job: %v", err)
	}
	return r.Create(ctx, pitrJob)
}

func (r *MariaDBReconciler) cleanupPITRJob(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	var job batchv1.Job
	if err := r.Get(ctx, mariadb.PITRJobKey(), &job); err == nil {
		if err := r.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)}); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (r *MariaDBReconciler) getStorageClient(ctx context.Context,
	pitr *mariadbv1alpha1.PointInTimeRecovery) (interfaces.BlobStorage, error) {
	storage := pitr.Spec.PointInTimeRecoveryStorage

	if storage.AzureBlob != nil {
		return r.getABSClient(ctx, pitr)
	}

	if storage.S3 != nil {
		return r.getS3Client(ctx, pitr)
	}

	return nil, fmt.Errorf("error getting a storage client, none configured. Either abs or s3 must be configure")

}

// getABSClient retrieves a configured Azure Blob Storage client
// This should not be used directly, see `getStorageClient`
func (r *MariaDBReconciler) getABSClient(ctx context.Context, pitr *mariadbv1alpha1.PointInTimeRecovery) (*azure.AzBlobClient, error) {
	abs := pitr.Spec.PointInTimeRecoveryStorage.AzureBlob
	if abs == nil {
		return nil, fmt.Errorf("error getting azure blob storage client. No abs config found")
	}

	opts := []azure.AzBlobOpt{
		azure.WithPrefix(abs.Prefix),
		azure.WithAccountName(abs.StorageAccountName),
	}

	// If `storageAccountKey` is not set, we rely on DefaultAzureCredential
	if abs.StorageAccountKey != nil {
		accountKey, err := r.RefResolver.SecretKeyRef(ctx, *abs.StorageAccountKey, pitr.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting CA cert: %v", err)
		}
		opts = append(opts, azure.WithAccountKey(accountKey))
	}

	tls := ptr.Deref(abs.TLS, mariadbv1alpha1.TLSConfig{})
	if tls.Enabled {
		opts = append(opts, azure.WithTLSEnabled(true))
		caCertBytes, err := r.RefResolver.SecretKeyRef(ctx, *abs.TLS.CASecretKeyRef, pitr.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting CA cert: %v", err)
		}
		opts = append(opts, azure.WithTLSCACertBytes([]byte(caCertBytes)))
	}

	return azure.NewAzBlobClient(
		"",
		abs.ContainerName,
		abs.ServiceURL,
		opts...,
	)
}

// getS3Client retrieves a configured S3 client
// @WARN: This should not be used directly, see `getStorageClient`
func (r *MariaDBReconciler) getS3Client(ctx context.Context, pitr *mariadbv1alpha1.PointInTimeRecovery) (*minio.Client, error) {
	s3 := pitr.Spec.PointInTimeRecoveryStorage.S3
	if s3 == nil {
		return nil, errors.New("error getting s3 client. No s3 config found")
	}

	return minio.NewMinioClientFromS3Config(
		ctx,
		*r.RefResolver,
		*s3,
		"",
		pitr.Namespace,
	)
}

func (r *MariaDBReconciler) shouldReconcilePITR(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, logger logr.Logger) (bool, error) {
	if mdb.IsInitializing() || mdb.IsUpdating() || mdb.IsRestoringBackup() || mdb.IsResizingStorage() ||
		mdb.IsScalingOut() || mdb.IsRecoveringReplicas() || mdb.HasGaleraNotReadyCondition() ||
		mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() {
		logger.V(1).Info("Operation in progress. Skipping PITR reconciliation...")
		return false, nil
	}
	if !mdb.HasRestoredPhysicalBackup() {
		logger.V(1).Info("PhysicalBackup not restored. Skipping PITR reconciliation...")
		return false, nil
	}
	if mdb.HasReplayedBinlogs() || mdb.HasSkippedBinlogReplay() {
		logger.V(1).Info("Binlogs already replayed or skipped. Skipping PITR reconciliation...")
		return false, nil
	}
	if mdb.Spec.BootstrapFrom == nil || mdb.Spec.BootstrapFrom.PointInTimeRecoveryRef == nil {
		return false, nil
	}

	healthy, err := health.IsStatefulSetHealthy(
		ctx,
		r.Client,
		client.ObjectKeyFromObject(mdb),
		health.WithDesiredReplicas(mdb.Spec.Replicas),
		health.WithPort(mdb.Spec.Port),
		health.WithEndpointPolicy(health.EndpointPolicyAll),
	)
	if err != nil {
		return false, fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		logger.V(1).Info("Some MariaDB Pods are not ready. Skipping PITR reconciliation...")
		return false, nil
	}
	return true, nil
}

func (r *MariaDBReconciler) cleanupPITRStagingPVC(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if !shouldProvisionPITRStagingPVC(mariadb) {
		return nil
	}
	key := mariadb.BootstrapFromStagingPVCKey()
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, key, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, &pvc)
}

func shouldProvisionPITRStagingPVC(mariadb *mariadbv1alpha1.MariaDB) bool {
	b := mariadb.Spec.BootstrapFrom
	if b == nil {
		return false
	}
	return b.PointInTimeRecoveryRef != nil && b.StagingStorage != nil && b.StagingStorage.PersistentVolumeClaim != nil
}
