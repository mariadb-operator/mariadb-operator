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
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/binlog"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/health"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

func (r *MariaDBReconciler) reconcilePITR(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("pitr")
	shouldReconcile, err := r.shouldReconcilePITR(ctx, mdb, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error determining whether PITR should be reconciled: %v", err)
	}
	if !shouldReconcile {
		return ctrl.Result{}, nil
	}

	startGtid, err := r.getStartGtid(ctx, mdb, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting start GTID: %v", err)
	}
	logger = logger.WithValues(
		"start-gtid", startGtid,
		"target-time", mdb.Spec.BootstrapFrom.TargetRecoveryTimeOrDefault().Format(time.RFC3339),
	)
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

	sqlClient, err := sql.NewClientWithMariaDB(ctx, mdb, r.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting SQL client: %v", err)
	}
	defer sqlClient.Close()
	if err := r.pauseGtidStrictMode(ctx, mdb, sqlClient, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error pausing gtid_strict_mode: %v", err)
	}

	if err := r.reconcilePITRStagingPVC(ctx, mdb); err != nil {
		return ctrl.Result{}, err
	}
	if result, err := r.reconcileAndWaitForPITRJob(ctx, mdb, startGtid, logger); !result.IsZero() || err != nil {
		return result, err
	}

	if err := r.resumeGtidStrictMode(ctx, mdb, sqlClient, logger); err != nil {
		return ctrl.Result{}, fmt.Errorf("error resuming gtid_strict_mode: %v", err)
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

		// TODO: handle galera, as the agent will not have this endpoint available
		agentGtid, err := agentClient.Replication.GetGtid(ctx)
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
	gtid, err := replication.ParseGtidWithDomainId(rawGtid, *domainId, logger.WithName("gtid"))
	if err != nil {
		return nil, fmt.Errorf("error parsing GTID %s: %v", rawGtid, err)
	}
	return gtid, nil
}

func (r *MariaDBReconciler) reconcileReplayBinlogsError(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, startGtid *replication.Gtid,
	logger logr.Logger) (ctrl.Result, error) {
	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mariadb.Spec.BootstrapFrom.PointInTimeRecoveryRef, mariadb.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting PointInTimeRecoveryRef: %v", err)
	}
	s3Client, err := r.getS3Client(ctx, pitr)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting S3 client: %v", err)
	}
	val, err := s3Client.GetCredentials().GetWithContext(nil)
	// S3 credentials are not static or AWS env variables are not set in the operator Pod.
	if err != nil || val == (credentials.Value{}) {
		logger.Info("Object storage credentials not found. Skipping binlog timeline validation...", "err", err)
		return ctrl.Result{}, nil
	}

	logger.Info("Validating binlog timeline")
	if err := r.validateBinlogTimeline(ctx, mariadb, startGtid, pitr.Spec.StrictMode, s3Client, logger); err != nil {
		errMsg := fmt.Sprintf("Invalid binary log timeline: %v", err)
		r.Recorder.Event(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonBinlogTimelineInvalid, errMsg)

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

func (r *MariaDBReconciler) validateBinlogTimeline(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, startGtid *replication.Gtid,
	strictMode bool, s3Client *minio.Client, logger logr.Logger) error {
	indexReader, err := s3Client.GetObjectWithOptions(ctx, binlog.BinlogIndexName)
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
			"error getting binlog timeline between GTID %s and target time %s: %v",
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

func (r *MariaDBReconciler) pauseGtidStrictMode(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, sqlClient *sql.Client,
	logger logr.Logger) error {
	pitr := ptr.Deref(mdb.Status.PointInTimeRecovery, mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{})
	if pitr.GtidStrictModePaused != nil && *pitr.GtidStrictModePaused {
		return nil
	}

	gtidStrictMode, err := sqlClient.GtidStrictMode(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_strict_mode: %v", err)
	}
	if !gtidStrictMode {
		return nil
	}

	logger.Info("Temporarily disabling gtid_strict_mode to replay binlogs")
	if err := sqlClient.DisableGtidStrictMode(ctx); err != nil {
		return fmt.Errorf("error disabling gtid_strict_mode: %v", err)
	}
	return r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if status.PointInTimeRecovery == nil {
			status.PointInTimeRecovery = &mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{}
		}
		status.PointInTimeRecovery.GtidStrictModePaused = ptr.To(true)
		return nil
	})
}

func (r *MariaDBReconciler) resumeGtidStrictMode(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, sqlClient *sql.Client,
	logger logr.Logger) error {
	pitr := ptr.Deref(mdb.Status.PointInTimeRecovery, mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{})
	if pitr.GtidStrictModePaused == nil || !*pitr.GtidStrictModePaused {
		return nil
	}

	logger.Info("Enabling back gtid_strict_mode")
	if err := sqlClient.EnableGtidStrictMode(ctx); err != nil {
		return fmt.Errorf("error enabling gtid_strict_mode: %v", err)
	}
	return r.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if status.PointInTimeRecovery != nil {
			status.PointInTimeRecovery.GtidStrictModePaused = nil
		}
		return nil
	})
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
	pitr, err := r.RefResolver.PointInTimeRecovery(ctx, mdb.Spec.BootstrapFrom.PointInTimeRecoveryRef, mdb.Namespace)
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

func (r *MariaDBReconciler) getS3Client(ctx context.Context, pitr *mariadbv1alpha1.PointInTimeRecovery) (*minio.Client, error) {
	s3 := pitr.Spec.S3
	minioOpts := []minio.MinioOpt{
		minio.WithRegion(s3.Region),
		minio.WithPrefix(s3.Prefix),
	}

	if s3.AccessKeyIdSecretKeyRef != nil && s3.SecretAccessKeySecretKeyRef != nil {
		accessKeyId, err := r.RefResolver.SecretKeyRef(ctx, *s3.AccessKeyIdSecretKeyRef, pitr.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting S3 access key ID: %v", err)
		}
		secretAccessKey, err := r.RefResolver.SecretKeyRef(ctx, *s3.SecretAccessKeySecretKeyRef, pitr.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting S3 access key ID: %v", err)
		}
		var sessionToken string
		if s3.SessionTokenSecretKeyRef != nil {
			sessionToken, err = r.RefResolver.SecretKeyRef(ctx, *s3.SessionTokenSecretKeyRef, pitr.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error getting S3 session token: %v", err)
			}
		}
		minioOpts = append(minioOpts, minio.WithCredsProviders(&credentials.Static{
			Value: credentials.Value{
				AccessKeyID:     accessKeyId,
				SecretAccessKey: secretAccessKey,
				SessionToken:    sessionToken,
				SignerType:      credentials.SignatureDefault,
			},
		}))
	}

	tls := ptr.Deref(s3.TLS, mariadbv1alpha1.TLSS3{})
	if tls.Enabled {
		minioOpts = append(minioOpts, minio.WithTLS(true))
		caCertBytes, err := r.RefResolver.SecretKeyRef(ctx, *s3.TLS.CASecretKeyRef, pitr.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting CA cert: %v", err)
		}
		minioOpts = append(minioOpts, minio.WithCACertBytes([]byte(caCertBytes)))
	}

	if s3.SSEC != nil {
		ssecKey, err := r.RefResolver.SecretKeyRef(ctx, s3.SSEC.CustomerKeySecretKeyRef, pitr.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting SSEC key: %v", err)
		}
		minioOpts = append(minioOpts, minio.WithSSECCustomerKey(ssecKey))
	}

	s3Client, err := minio.NewMinioClient(
		"", // not needed: in-memory methods (io.Reader instead of a file) are used in this context
		pitr.Spec.S3.Bucket,
		pitr.Spec.S3.Endpoint,
		minioOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 client: %v", err)
	}
	return s3Client, nil
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
	if mdb.HasReplayedBinlogs() || mdb.Spec.BootstrapFrom == nil || mdb.Spec.BootstrapFrom.PointInTimeRecoveryRef == nil {
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

func shouldProvisionPITRStagingPVC(mariadb *mariadbv1alpha1.MariaDB) bool {
	b := mariadb.Spec.BootstrapFrom
	if b == nil {
		return false
	}
	return b.PointInTimeRecoveryRef != nil && b.StagingStorage != nil && b.StagingStorage.PersistentVolumeClaim != nil
}
