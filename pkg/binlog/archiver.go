package binlog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	mariadbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	conditions "github.com/mariadb-operator/mariadb-operator/v26/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	BinlogIndexName        = "index.yaml"
	archiveInterval        = 10 * time.Minute
	defaultArchivalTimeout = metav1.Duration{Duration: time.Hour}
)

type Archiver struct {
	dataDir     string
	env         *environment.PodEnvironment
	client      client.Client
	refResolver *refresolver.RefResolver
	recorder    record.EventRecorder
	logger      logr.Logger
}

func NewArchiver(dataDir string, env *environment.PodEnvironment, client client.Client,
	recorder record.EventRecorder, logger logr.Logger) *Archiver {
	return &Archiver{
		dataDir:     dataDir,
		env:         env,
		client:      client,
		refResolver: refresolver.New(client),
		recorder:    recorder,
		logger:      logger,
	}
}

func (a *Archiver) Start(ctx context.Context) error {
	a.logger.Info("Starting binary log archiver")

	ticker := time.NewTicker(archiveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Stopping binary log archiver")
			return nil
		case <-ticker.C:
			mdb, err := a.getMariaDB(ctx)
			if err != nil {
				return fmt.Errorf("error getting MariaDB: %v", err)
			}
			if !a.shouldArchiveBinlogs(mdb) {
				continue
			}
			archiveErr := a.archiveBinaryLogs(ctx, mdb)

			if err := a.updateStatusWithError(ctx, mdb, archiveErr); err != nil {
				return fmt.Errorf("error updating status with error: %v", err)
			}
		}
	}
}

func (a *Archiver) shouldArchiveBinlogs(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.Status.CurrentPrimary == nil ||
		(mdb.Status.CurrentPrimary != nil && *mdb.Status.CurrentPrimary != a.env.PodName) {
		a.logger.V(1).Info("Current primary not set or current Pod is a replica, skipping binary log archival...")
		return false
	}
	if mdb.IsRestoringBackup() {
		a.logger.Info("Backup restoration in progress, skipping binary log archival...")
		return false
	}
	if mdb.IsInitializing() {
		a.logger.Info("Initialization in progress, skipping binary log archival...")
		return false
	}
	if mdb.IsSwitchingPrimary() || mdb.IsReplicationSwitchoverRequired() {
		a.logger.Info("Switchover operation pending/ongoing, skipping binary log archival...")
		return false
	}
	if mdb.IsUpdating() || mdb.HasPendingUpdate() {
		a.logger.Info("Update in progress, skipping binary log archival...")
		return false
	}
	if mdb.IsResizingStorage() {
		a.logger.Info("Storage resize in progress, skipping binary log archival...")
		return false
	}
	if mdb.IsRecoveringReplicas() {
		a.logger.Info("Replica recovery in progress, skipping binary log archival...")
		return false
	}
	if mdb.HasGaleraNotReadyCondition() {
		a.logger.Info("Galera not ready, skipping binary log archival...")
		return false
	}
	return true
}

func (a *Archiver) archiveBinaryLogs(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) error {
	a.logger.Info("Archiving binary logs")
	pitr, err := a.getPointInTimeRecovery(ctx, mdb)
	if err != nil {
		return err
	}
	s3Client, err := a.getS3Client(&pitr.Spec.S3, a.env)
	if err != nil {
		return err
	}

	storageAlreadyInit, err := a.checkStorageAlreadyInitialized(ctx, mdb, s3Client)
	if err != nil {
		return fmt.Errorf("error checking whether storage is already initialized: %v", err)
	}
	if storageAlreadyInit {
		return errors.New("binary log storage is already initialized. Archival must start from a clean state")
	}

	isConfigured, err := a.physicalBackupConfigured(ctx, &pitr.Spec.PhysicalBackupRef, mdb)
	if err != nil {
		return fmt.Errorf("error checking PhysicalBackup: %v", err)
	}
	if !isConfigured {
		return errors.New("PhysicalBackup not configured. Skipping binary log archival...") //nolint:staticcheck
	}

	sqlClient, err := sql.NewLocalClientWithPodEnv(ctx, a.env)
	if err != nil {
		return fmt.Errorf("error getting SQL client: %v", err)
	}
	defer sqlClient.Close()

	binlogs, err := a.getBinaryLogs(ctx, sqlClient)
	if err != nil {
		return fmt.Errorf("error getting binary logs: %v", err)
	}
	if len(binlogs) == 0 {
		return errors.New("no binary logs were found")
	}
	if len(binlogs) == 1 {
		a.logger.V(1).Info("Only active binary log is available. Skipping binary log archival...")
		return nil
	}
	// skip active binary log
	binlogs = binlogs[:len(binlogs)-1]

	if err := a.resetArchivedBinlog(ctx, binlogs, mdb); err != nil {
		return fmt.Errorf("error resetting binary logs: %v", err)
	}

	compressor, err := a.getCompressor(pitr.Spec.Compression)
	if err != nil {
		return err
	}
	uploader := NewUploader(
		a.dataDir,
		s3Client,
		compressor,
		a.logger.WithName("uploader"),
	)

	timeOutCtx, cancel := context.WithTimeout(ctx, ptr.Deref(pitr.Spec.ArchiveTimeout, defaultArchivalTimeout).Duration)
	defer cancel()

	for i := 0; i < len(binlogs); i++ {
		select {
		case <-timeOutCtx.Done():
			return fmt.Errorf("archival timed out: %w", timeOutCtx.Err())
		default:
			if err := a.archiveBinaryLog(timeOutCtx, binlogs[i], mdb, pitr, uploader); err != nil {
				return err
			}
		}
	}
	a.logger.Info("Binlog archival done")

	return a.updateStatus(ctx, binlogs, s3Client, sqlClient)
}

func (a *Archiver) getMariaDB(ctx context.Context) (*mariadbv1alpha1.MariaDB, error) {
	key := types.NamespacedName{
		Name:      a.env.MariadbName,
		Namespace: a.env.PodNamespace,
	}
	var mdb mariadbv1alpha1.MariaDB
	if err := a.client.Get(ctx, key, &mdb); err != nil {
		return nil, fmt.Errorf("error getting MariaDB: %v", err)
	}
	return &mdb, nil
}

func (a *Archiver) getPointInTimeRecovery(ctx context.Context, mdb *mariadbv1alpha1.MariaDB) (*mariadbv1alpha1.PointInTimeRecovery, error) {
	if !mdb.IsPointInTimeRecoveryEnabled() {
		return nil, errors.New("point-in-time recovery must be enabled in MariaDB")
	}
	pitr, err := a.refResolver.PointInTimeRecovery(ctx, mdb.Spec.PointInTimeRecoveryRef, mdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	return pitr, nil
}

func (a *Archiver) getS3Client(s3 *mariadbv1alpha1.S3, env *environment.PodEnvironment) (*mariadbminio.Client, error) {
	tls := ptr.Deref(s3.TLS, mariadbv1alpha1.TLSS3{})
	minioOpts := []mariadbminio.MinioOpt{
		mariadbminio.WithTLS(tls.Enabled),
		mariadbminio.WithRegion(s3.Region),
		mariadbminio.WithPrefix(s3.Prefix),
		mariadbminio.WithAllowNestedPrefixes(true),
	}
	if env.MariadbOperatorS3CAPath != "" {
		minioOpts = append(minioOpts, mariadbminio.WithCACertPath(env.MariadbOperatorS3CAPath))
	}
	// TODO: support for SSEC based on environment

	client, err := mariadbminio.NewMinioClient(a.dataDir, s3.Bucket, s3.Endpoint, minioOpts...)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 client: %v", err)
	}
	return client, nil
}

func (a *Archiver) getCompressor(calg mariadbv1alpha1.CompressAlgorithm) (mariadbcompression.Compressor, error) {
	if calg == mariadbv1alpha1.CompressAlgorithm("") {
		calg = mariadbv1alpha1.CompressNone
	}
	if err := calg.Validate(); err != nil {
		return nil, fmt.Errorf("compression algorithm not supported: %v", err)
	}
	return mariadbcompression.NewCompressor(calg)
}

func (a *Archiver) checkStorageAlreadyInitialized(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	s3Client *mariadbminio.Client) (bool, error) {
	if mdb.Status.PointInTimeRecovery != nil {
		return false, nil
	}
	objs, err := s3Client.ListObjectsWithOptions(ctx)
	if err != nil {
		return false, err
	}
	return len(objs) > 0, nil
}

func (a *Archiver) physicalBackupConfigured(ctx context.Context, ref *mariadbv1alpha1.LocalObjectReference,
	mdb *mariadbv1alpha1.MariaDB) (bool, error) {
	key := types.NamespacedName{
		Name:      ref.Name,
		Namespace: mdb.Namespace,
	}
	var physicalBackup mariadbv1alpha1.PhysicalBackup
	if err := a.client.Get(ctx, key, &physicalBackup); err != nil {
		return false, err
	}
	return true, nil
}

func (a *Archiver) getBinaryLogs(ctx context.Context, sqlClient *sql.Client) ([]string, error) {
	binaryLogIndex, err := sqlClient.BinaryLogIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting binary log index: %v", err)
	}

	file, err := os.Open(binaryLogIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to open binlog index: %v", err)
	}
	defer file.Close()

	var binlogs []string
	fileScanner := bufio.NewScanner(file)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		binlog := path.Base(fileScanner.Text())
		binlogs = append(binlogs, binlog)
	}
	sort.Strings(binlogs)

	return binlogs, nil
}

func (a *Archiver) resetArchivedBinlog(ctx context.Context, binlogs []string, mdb *mariadbv1alpha1.MariaDB) error {
	if mdb.Status.PointInTimeRecovery == nil {
		return nil
	}
	meta, err := GetBinlogMetadata(filepath.Join(a.dataDir, binlogs[0]), a.logger)
	if err != nil {
		return fmt.Errorf("error getting binary log %s metadata: %v", binlogs[0], err)
	}

	if mdb.Status.PointInTimeRecovery.ServerId != meta.ServerId {
		a.logger.Info(
			"Detected server_id change. Resetting binary log archival status...",
			"server-id", mdb.Status.PointInTimeRecovery.ServerId,
			"new-server-id", meta.ServerId,
		)
		if err := a.patchMariadbStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			// setting this to empty empty struct, as setting it to nil
			// would trigger an error when checking binlog storage initialization
			status.PointInTimeRecovery = &mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{}
		}); err != nil {
			return fmt.Errorf("error patching MariaDB: %v", err)
		}
	}

	return nil
}

func (a *Archiver) archiveBinaryLog(ctx context.Context, binlog string, mdb *mariadbv1alpha1.MariaDB,
	pitr *mariadbv1alpha1.PointInTimeRecovery, uploader *Uploader) error {
	a.logger.V(1).Info("Processing binary log", "binlog", binlog)

	if mdb.Status.PointInTimeRecovery != nil && mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog != "" {
		num, err := ParseBinlogNum(binlog)
		if err != nil {
			return fmt.Errorf("error parsing binlog number in %s: %v", binlog, err)
		}
		archivedNum, err := ParseBinlogNum(mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog)
		if err != nil {
			return fmt.Errorf("error archiving parsing binlog number in %s: %v", mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog, err)
		}

		if num.LessThan(archivedNum) || num.Equal(archivedNum) {
			a.logger.V(1).Info("Skipping binary log since a more recent one has already been archived", "binlog", binlog)
			return nil
		}
	}

	if err := uploader.Upload(ctx, binlog, mdb, pitr); err != nil {
		return fmt.Errorf("error uploading binary log %s: %v", binlog, err)
	}
	msg := fmt.Sprintf("Binary log %s archived", binlog)
	a.logger.Info(msg)
	a.recorder.Event(mdb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonBinlogArchived, msg)

	return nil
}

func (a *Archiver) updateStatus(ctx context.Context, binlogs []string, s3Client *mariadbminio.Client,
	sqlClient *sql.Client) error {
	mdb, err := a.getMariaDB(ctx)
	if err != nil {
		return err
	}
	pitr, err := a.getPointInTimeRecovery(ctx, mdb)
	if err != nil {
		return err
	}
	backup, err := a.refResolver.PhysicalBackup(ctx, &pitr.Spec.PhysicalBackupRef, a.env.PodNamespace)
	if err != nil {
		return fmt.Errorf("error getting PhysicalBackup: %v", err)
	}
	gtidDomainId, err := sqlClient.GtidDomainId(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_domain_id: %v", err)
	}

	pitrStatus, err := a.getPointInTimeRecoveryStatus(binlogs[len(binlogs)-1], *gtidDomainId)
	if err != nil {
		return fmt.Errorf("error getting PITR status: %v", err)
	}
	binlogIndex, err := a.updateBinlogIndex(ctx, binlogs, pitrStatus.ServerId, s3Client)
	if err != nil {
		return fmt.Errorf("error updating binlog index: %v", err)
	}
	lastRecoverableTime, err := a.getLastRecoverableTime(binlogIndex, backup, *gtidDomainId)
	if err != nil {
		return fmt.Errorf("error getting last recoverable time: %v", err)
	}

	if err := a.patchMariadbStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.PointInTimeRecovery = pitrStatus
	}); err != nil {
		return fmt.Errorf("error patching MariaDB PITR status: %v", err)
	}
	if lastRecoverableTime != nil {
		if err := a.patchPITRStatus(ctx, pitr, func(status *mariadbv1alpha1.PointInTimeRecoveryStatus) {
			status.LastRecoverableTime = ptr.To(lastRecoverableTime.Format(time.RFC3339))
		}); err != nil {
			return fmt.Errorf("error patching PITR status: %v", err)
		}
	}
	return nil
}

func (a *Archiver) updateStatusWithError(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, archiveErr error) error {
	if archiveErr != nil {
		a.logger.Error(archiveErr, "Error archiving binary logs")

		if err := a.patchMariadbStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			conditions.SetArchivedBinlogsError(status, archiveErr.Error())
		}); err != nil {
			return fmt.Errorf("error patching MariaDB status: %v", err)
		}
		a.recorder.Eventf(mdb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonBinlogArchivalError,
			"Error archiving binary logs: %v", archiveErr)
	} else {
		if err := a.patchMariadbStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			conditions.SetArchivedBinlogs(status)
		}); err != nil {
			return fmt.Errorf("error patching MariaDB status: %v", err)
		}
	}
	return nil
}

func (a *Archiver) getPointInTimeRecoveryStatus(lastBinlog string,
	gtidDomainId uint32) (*mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus, error) {
	lastBinlogPath := filepath.Join(a.dataDir, lastBinlog)
	lastBinlogMeta, err := GetBinlogMetadata(lastBinlogPath, a.logger)
	if err != nil {
		return nil, fmt.Errorf("error getting last binlog %s metadata: %v", lastBinlog, err)
	}

	lastArchivedGtid := lastBinlogMeta.LastGtid
	if lastArchivedGtid == nil && len(lastBinlogMeta.PreviousGtids) > 0 {
		for _, gtid := range lastBinlogMeta.PreviousGtids {
			if gtid.DomainID == gtidDomainId {
				lastArchivedGtid = gtid
				break
			}
		}
	}

	return &mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{
		ServerId:              lastBinlogMeta.ServerId,
		LastArchivedBinaryLog: lastBinlog,
		LastArchivedTime:      lastBinlogMeta.LastTime,
		LastArchivedPosition:  lastBinlogMeta.LogPosition,
		LastArchivedGtid:      lastArchivedGtid,
	}, nil
}

func (a *Archiver) updateBinlogIndex(ctx context.Context, binlogs []string, serverId uint32,
	s3Client *mariadbminio.Client) (*BinlogIndex, error) {
	var index *BinlogIndex
	exists, err := s3Client.Exists(ctx, BinlogIndexName)
	if err != nil {
		return nil, fmt.Errorf("error checking if binlog index exists: %v", err)
	}
	if exists {
		indexReader, err := s3Client.GetObjectWithOptions(ctx, BinlogIndexName)
		if err != nil {
			return nil, fmt.Errorf("error getting binlog index: %v", err)
		}
		defer indexReader.Close()

		bytes, err := io.ReadAll(indexReader)
		if err != nil {
			return nil, fmt.Errorf("error reading binlog index: %v", err)
		}
		var bi BinlogIndex
		if err := yaml.Unmarshal(bytes, &bi); err != nil {
			return nil, fmt.Errorf("error unmarshaling binlog index: %v", err)
		}
		index = &bi
	} else {
		index = NewBinlogIndex()
	}

	for _, binlog := range binlogs {
		if index.Exists(serverId, binlog) {
			a.logger.V(1).Info("binlog already present in index. Skipping...", "binlog", binlog)
			continue
		}

		meta, err := GetBinlogMetadata(filepath.Join(a.dataDir, binlog), a.logger)
		if err != nil {
			return nil, fmt.Errorf("error getting binlog %s metadata: %v", binlog, err)
		}
		index.Add(serverId, *meta)
	}

	indexBytes, err := yaml.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("error marshaling binlog index: %v", err)
	}

	if err := s3Client.PutObjectWithOptions(ctx, BinlogIndexName, bytes.NewReader(indexBytes), int64(len(indexBytes))); err != nil {
		return nil, fmt.Errorf("error putting binlog index: %v", err)
	}
	return index, nil
}

func (a *Archiver) getLastRecoverableTime(binlogIndex *BinlogIndex, backup *mariadbv1alpha1.PhysicalBackup,
	gtidDomainId uint32) (*metav1.Time, error) {
	lastGtid, ok := backup.Annotations[metadata.LastGtidAnnotation]
	if !ok {
		a.logger.Info(
			"Last GTID annotation not found in PhysicalBackup. Skipping last recoverable time tracking...",
			"physicalbackup", backup.Name,
		)
		return nil, nil
	}
	gtid, err := replication.ParseGtidWithDomainId(lastGtid, gtidDomainId, a.logger)
	if err != nil {
		return nil, fmt.Errorf("error parsing GTID: %v", err)
	}

	// getting the most recent timeline, disabling strict mode to prevent errors.
	// last recoverable time will be checked before bootstrapping, returning error if strict mode is enabled.
	binlogMetas, err := binlogIndex.BuildTimeline(
		gtid,
		time.Now(),
		false,
		a.logger.WithName("binlog-timeline").V(1),
	)
	if err != nil {
		a.logger.Info(
			"Unable to build current binlog timeline. Skipping last recoverable time tracking...",
			"err", err,
			"gtid", gtid.String(),
			"physicalbackup", backup.Name,
		)
		return nil, nil
	}
	lastMeta := binlogMetas[len(binlogMetas)-1]

	return &lastMeta.LastTime, nil
}

func (a *Archiver) patchMariadbStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return a.client.Status().Patch(ctx, mariadb, patch)
}

func (a *Archiver) patchPITRStatus(ctx context.Context, pitr *mariadbv1alpha1.PointInTimeRecovery,
	patcher func(*mariadbv1alpha1.PointInTimeRecoveryStatus)) error {
	patch := client.MergeFrom(pitr.DeepCopy())
	patcher(&pitr.Status)
	return a.client.Status().Patch(ctx, pitr, patch)
}
