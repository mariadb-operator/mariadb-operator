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
	"strings"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	mariadbcompression "github.com/mariadb-operator/mariadb-operator/v25/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	BinlogIndexName = "index.yaml"
	archiveInterval = 10 * time.Minute
)

type Archiver struct {
	dataDir string
	env     *environment.PodEnvironment
	client  client.Client
	logger  logr.Logger
}

func NewArchiver(dataDir string, env *environment.PodEnvironment, client *client.Client,
	logger logr.Logger) *Archiver {
	return &Archiver{
		dataDir: dataDir,
		env:     env,
		client:  *client,
		logger:  logger,
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
			if err := a.archiveBinaryLogs(ctx); err != nil {
				a.logger.Error(err, "Error archiving binary logs")
			}
		}
	}
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
	if mdb.Spec.PointInTimeRecoveryRef == nil {
		return nil, errors.New("'spec.pointInTimeRecoveryRef' must be set in MariaDB object")
	}
	key := types.NamespacedName{
		Name:      mdb.Spec.PointInTimeRecoveryRef.Name,
		Namespace: a.env.PodNamespace,
	}
	var pitr mariadbv1alpha1.PointInTimeRecovery
	if err := a.client.Get(ctx, key, &pitr); err != nil {
		return nil, fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	return &pitr, nil
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
	return mariadbcompression.NewCompressor(calg, a.dataDir, getUncompressedBinlog, a.logger)
}

func (a *Archiver) archiveBinaryLogs(ctx context.Context) error {
	mdb, err := a.getMariaDB(ctx)
	if err != nil {
		return err
	}
	if mdb.Status.CurrentPrimary == nil ||
		(mdb.Status.CurrentPrimary != nil && *mdb.Status.CurrentPrimary != a.env.PodName) {
		a.logger.V(1).Info("Current primary not set or current Pod is a replica. Skipping binary log archival...")
		return nil
	}
	// TODO: fine grained guard
	if mdb.IsSwitchingPrimary() {
		return errors.New("unable to start archival: Switchover operation pending/ongoing")
	}
	a.logger.Info("Archiving binary logs")

	pitr, err := a.getPointInTimeRecovery(ctx, mdb)
	if err != nil {
		return err
	}

	isConfigured, err := a.physicalBackupConfigured(ctx, &pitr.Spec.PhysicalBackupRef, mdb)
	if err != nil {
		return fmt.Errorf("error checking PhysicalBackup: %v", err)
	}
	if !isConfigured {
		return errors.New("PhysicalBackup not configured, stopping binary log archival") //nolint:staticcheck
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

	s3Client, err := a.getS3Client(&pitr.Spec.S3, a.env)
	if err != nil {
		return err
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

	for i := 0; i < len(binlogs); i++ {
		if err := a.archiveBinaryLog(ctx, binlogs[i], mdb, pitr, uploader); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, binlogs, s3Client, sqlClient)
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
		return nil, fmt.Errorf("failed to open binlog index: %w", err)
	}

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
		if err := a.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.PointInTimeRecovery = nil
		}); err != nil {
			return fmt.Errorf("error patching MariaDB: %v", err)
		}
	}

	return nil
}

func (a *Archiver) archiveBinaryLog(ctx context.Context, binlog string, mdb *mariadbv1alpha1.MariaDB,
	pitr *mariadbv1alpha1.PointInTimeRecovery, uploader *Uploader) error {
	a.logger.V(1).Info("Processing binary log", "binlog", binlog)

	if mdb.Status.PointInTimeRecovery != nil {
		num, err := ParseBinlogNum(binlog)
		if err != nil {
			return fmt.Errorf("error parsing binlog number in %s: %v", binlog, err)
		}
		archivedNum, err := ParseBinlogNum(mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog)
		if err != nil {
			return fmt.Errorf("error archived parsing binlog number in %s: %v", mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog, err)
		}

		if num.LessThan(archivedNum) || num.Equal(archivedNum) {
			a.logger.V(1).Info("Skipping binary log since a more recent one has already been archived", "binlog", binlog)
			return nil
		}
	}

	if err := uploader.Upload(ctx, binlog, mdb, pitr); err != nil {
		return fmt.Errorf("error uploading binary log %s: %v", binlog, err)
	}
	return nil
}

func (a *Archiver) updateStatus(ctx context.Context, binlogs []string, s3Client *mariadbminio.Client,
	sqlClient *sql.Client) error {
	mdb, err := a.getMariaDB(ctx)
	if err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	// using last binlog to track the most recent GTIDs
	pitrStatus, err := a.getPointInTimeRecoveryStatus(ctx, binlogs[len(binlogs)-1], sqlClient)
	if err != nil {
		return fmt.Errorf("error getting PITR status: %v", err)
	}

	if err := a.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
		status.PointInTimeRecovery = pitrStatus
	}); err != nil {
		return fmt.Errorf("error updating PITR status: %v", err)
	}
	if err := a.updateBinlogIndex(ctx, binlogs, pitrStatus.ServerId, s3Client); err != nil {
		return fmt.Errorf("error updating binlog index: %v", err)
	}
	return nil
}

func (a *Archiver) getPointInTimeRecoveryStatus(ctx context.Context, lastBinlog string,
	sqlClient *sql.Client) (*mariadbv1alpha1.PointInTimeRecoveryStatus, error) {
	lastBinlogPath := filepath.Join(a.dataDir, lastBinlog)
	lastBinlogMeta, err := GetBinlogMetadata(lastBinlogPath, a.logger)
	if err != nil {
		return nil, fmt.Errorf("error getting last binlog %s metadata: %v", lastBinlog, err)
	}

	lastArchivedGtid := lastBinlogMeta.LastGtid
	if lastArchivedGtid == nil && len(lastBinlogMeta.PreviousGtids) > 0 {
		domainId, err := sqlClient.GtidDomainId(ctx)
		if err != nil {
			a.logger.Error(err, "error getting domain ID")
		}
		if domainId != nil {
			for _, gtid := range lastBinlogMeta.PreviousGtids {
				if gtid.DomainID == *domainId {
					lastArchivedGtid = gtid
					break
				}
			}
		}
	}

	return &mariadbv1alpha1.PointInTimeRecoveryStatus{
		ServerId:              lastBinlogMeta.ServerId,
		LastArchivedBinaryLog: lastBinlog,
		LastArchivedTime:      lastBinlogMeta.LastTime,
		LastArchivedPosition:  lastBinlogMeta.LogPosition,
		LastArchivedGtid:      lastArchivedGtid,
	}, nil
}

func (a *Archiver) updateBinlogIndex(ctx context.Context, binlogs []string, serverId uint32, s3Client *mariadbminio.Client) error {
	var index *BinlogIndex
	exists, err := s3Client.Exists(ctx, BinlogIndexName)
	if err != nil {
		return fmt.Errorf("error checking if binlog index exists: %v", err)
	}
	if exists {
		indexReader, err := s3Client.GetObjectWithOptions(ctx, BinlogIndexName)
		if err != nil {
			return fmt.Errorf("error getting binlog index: %w", err)
		}
		defer indexReader.Close()

		bytes, err := io.ReadAll(indexReader)
		if err != nil {
			return fmt.Errorf("error reading binlog index: %w", err)
		}
		var bi BinlogIndex
		if err := yaml.Unmarshal(bytes, &bi); err != nil {
			return fmt.Errorf("error unmarshaling binlog index: %w", err)
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
			return fmt.Errorf("error getting binlog %s metadata: %v", binlog, err)
		}
		index.Add(serverId, *meta)
	}

	indexBytes, err := yaml.Marshal(index)
	if err != nil {
		return fmt.Errorf("error marshaling binlog index: %v", err)
	}

	if err := s3Client.PutObjectWithOptions(ctx, BinlogIndexName, bytes.NewReader(indexBytes), int64(len(indexBytes))); err != nil {
		return fmt.Errorf("error putting binlog index: %v", err)
	}
	return nil
}

func (a *Archiver) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return a.client.Status().Patch(ctx, mariadb, patch)
}

func getUncompressedBinlog(compressedBinlog string) (string, error) {
	// compressed binlog format: mariadb-bin.000001.bz2
	parts := strings.Split(compressedBinlog, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid compressed binlog file name: %s", compressedBinlog)
	}

	calg := mariadbv1alpha1.CompressAlgorithm(parts[2])
	if err := calg.Validate(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", parts[0], parts[1]), nil
}
