package binlog

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	mariadbcompression "github.com/mariadb-operator/mariadb-operator/v25/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var archiveInterval = 10 * time.Minute

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
	}
	if env.MariadbOperatorS3CAPath != "" {
		minioOpts = append(minioOpts, mariadbminio.WithCACertPath(env.MariadbOperatorS3CAPath))
	}

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
	if mdb.IsSwitchoverRequired() || mdb.IsSwitchingPrimary() {
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
		if err := a.archiveBinaryLog(ctx, binlogs, i, uploader); err != nil {
			return err
		}
	}
	return nil
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
	return binlogs, nil
}

func (a *Archiver) resetArchivedBinlog(ctx context.Context, binlogs []string, mdb *mariadbv1alpha1.MariaDB) error {
	if len(binlogs) == 0 || mdb.Status.PointInTimeRecovery == nil || mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog == nil {
		return nil
	}
	prefix, err := ParseBinlogPrefix(binlogs[0])
	if err != nil {
		return fmt.Errorf("error parsing binlog prefix in %s: %v", binlogs[0], err)
	}
	archivedPrefix, err := ParseBinlogPrefix(*mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog)
	if err != nil {
		return fmt.Errorf("error parsing archived binlog prefix in %s: %v", *mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog, err)
	}

	lastNum, err := ParseBinlogNum(binlogs[len(binlogs)-1])
	if err != nil {
		return fmt.Errorf("error parsing last binlog number in %s: %v", binlogs[len(binlogs)-1], err)
	}
	archivedNum, err := ParseBinlogNum(*mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog)
	if err != nil {
		return fmt.Errorf("error parsing archived binlog number in %s: %v", *mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog, err)
	}

	if *prefix != *archivedPrefix || lastNum.LessThan(archivedNum) {
		a.logger.Info(
			"Resetting last archived binary log",
			"prefix", prefix,
			"archived-prefix", archivedPrefix,
			"last-num", lastNum,
			"archived-num", archivedNum,
		)
		if err := a.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.PointInTimeRecovery = nil
		}); err != nil {
			return fmt.Errorf("error patching MariaDB: %v", err)
		}
	}
	return nil
}

func (a *Archiver) archiveBinaryLog(ctx context.Context, binlogs []string, binlogIndex int, uploader *Uploader) error {
	if len(binlogs) == 0 {
		return errors.New("no binary logs were provided")
	}
	if binlogIndex < 0 || binlogIndex >= len(binlogs) {
		return fmt.Errorf("binary log index %d out of bounds [0, %d]", binlogIndex, len(binlogs)-1)
	}
	mdb, err := a.getMariaDB(ctx)
	if err != nil {
		return err
	}
	pitr, err := a.getPointInTimeRecovery(ctx, mdb)
	if err != nil {
		return err
	}
	binlog := binlogs[binlogIndex]
	a.logger.V(1).Info("Processing binary log", "binlog", binlog)

	if binlogIndex == len(binlogs)-1 {
		a.logger.V(1).Info("Skipping active binary log", "binlog", binlog)
		return nil
	}

	if mdb.Status.PointInTimeRecovery != nil && mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog != nil {
		num, err := ParseBinlogNum(binlog)
		if err != nil {
			return fmt.Errorf("error parsing binlog number in %s: %v", binlog, err)
		}
		archivedNum, err := ParseBinlogNum(*mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog)
		if err != nil {
			return fmt.Errorf("error archived parsing binlog number in %s: %v", *mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog, err)
		}

		if num.LessThan(archivedNum) || num.Equal(archivedNum) {
			a.logger.V(1).Info("Skipping binary log since a more recent one has already been archived", "binlog", binlog)
			return nil
		}
	}

	if err := uploader.Upload(ctx, binlog, mdb, pitr); err != nil {
		return fmt.Errorf("error uploading binary log %s: %v", binlog, err)
	}
	if err := a.updateLastArchivedBinlog(ctx, binlog); err != nil {
		return fmt.Errorf("error updating last archived with binlog %s: %v", binlog, err)
	}
	return nil
}

func (a *Archiver) updateLastArchivedBinlog(ctx context.Context, binlog string) error {
	num, err := ParseBinlogNum(binlog)
	if err != nil {
		return fmt.Errorf("error parsing current binary log number in %s: %v", binlog, err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mdb, err := a.getMariaDB(ctx)
		if err != nil {
			return err
		}
		if mdb.Status.PointInTimeRecovery == nil {
			return a.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
				status.PointInTimeRecovery = &mariadbv1alpha1.PointInTimeRecoveryStatus{
					LastArchivedBinaryLog: &binlog,
				}
			})
		}
		if mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog == nil {
			return a.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
				status.PointInTimeRecovery.LastArchivedBinaryLog = &binlog
			})
		}

		archivedNum, err := ParseBinlogNum(*mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog)
		if err != nil {
			return fmt.Errorf(
				"error parsing last archived binary log number in %s: %v",
				*mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog,
				err,
			)
		}
		if num.LessThan(archivedNum) || num.Equal(archivedNum) {
			return nil
		}

		return a.patchStatus(ctx, mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.PointInTimeRecovery.LastArchivedBinaryLog = &binlog
		})
	})
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
