package binlog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/minio/minio-go/v7"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var archiveInterval = 10 * time.Minute

type Archiver struct {
	fileManager *filemanager.FileManager
	env         *environment.PodEnvironment
	client      client.Client
	logger      logr.Logger
}

func NewArchiver(fileManager *filemanager.FileManager, env *environment.PodEnvironment, client *client.Client,
	logger logr.Logger) *Archiver {
	return &Archiver{
		fileManager: fileManager,
		env:         env,
		client:      *client,
		logger:      logger,
	}
}

func (a *Archiver) Start(ctx context.Context) error {
	a.logger.Info("Starting binary log archiver")

	mdb, err := a.getMariaDB(ctx)
	if err != nil {
		return err
	}
	pitr, err := a.getPointInTimeRecovery(ctx, mdb)
	if err != nil {
		return err
	}
	sqlClient, err := sql.NewLocalClientWithPodEnv(ctx, a.env)
	if err != nil {
		return fmt.Errorf("error getting SQL client: %v", err)
	}
	defer sqlClient.Close()
	// TODO: mount TLS certs and credentials
	s3Client, err := getS3Client(pitr)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(archiveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Stopping binary log archiver")
			return nil
		case <-ticker.C:
			if err := a.archiveBinaryLogs(ctx, mdb, pitr, sqlClient, s3Client); err != nil {
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
	if mdb.Spec.PointtInTimeRecoveryRef == nil {
		return nil, errors.New("'spec.pointInTimeRecoveryRef' must be set in MariaDB object")
	}
	key := types.NamespacedName{
		Name:      mdb.Spec.PointtInTimeRecoveryRef.Name,
		Namespace: a.env.PodNamespace,
	}
	var pitr mariadbv1alpha1.PointInTimeRecovery
	if err := a.client.Get(ctx, key, &pitr); err != nil {
		return nil, fmt.Errorf("error getting PointInTimeRecovery: %v", err)
	}
	return &pitr, nil
}

func (a *Archiver) archiveBinaryLogs(ctx context.Context, mdb *mariadbv1alpha1.MariaDB,
	pitr *mariadbv1alpha1.PointInTimeRecovery, sqlClient *sql.Client, s3Client *minio.Client) error {
	if mdb.Status.CurrentPrimary == nil ||
		(mdb.Status.CurrentPrimary != nil && *mdb.Status.CurrentPrimary != a.env.PodName) {
		return nil
	}
	if mdb.IsSwitchoverRequired() || mdb.IsSwitchingPrimary() {
		return errors.New("Unable to start archival: Switchover operation pending/ongoing")
	}
	a.logger.Info("Archiving binary logs")

	binlogs, err := a.getBinaryLogs(ctx, sqlClient)
	if err != nil {
		return fmt.Errorf("error getting binary logs: %v", err)
	}

	if len(binlogs) > 0 && mdb.Status.PointInTimeRecovery != nil && mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog != nil {
		shouldReset, err := shouldResetArchivedBinlog(
			binlogs,
			*mdb.Status.PointInTimeRecovery.LastArchivedBinaryLog,
			a.logger,
		)
		if err != nil {
			return fmt.Errorf("error checking archived binary log: %v", err)
		}
		if shouldReset {
			if err := a.patchStatus(ctx, mdb, func(mdb *mariadbv1alpha1.MariaDBStatus) {
				mdb.PointInTimeRecovery.LastArchivedBinaryLog = nil
			}); err != nil {
				return fmt.Errorf("error patching MariaDB: %v", err)
			}
			// TODO: verify  if additional request to get updated MariaDB is needed (it shouldm't)
		}
	}

	return nil
}

func (a *Archiver) getBinaryLogs(ctx context.Context, sqlClient *sql.Client) ([]string, error) {
	binaryLogIndex, err := sqlClient.BinaryLogIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting binary log index: %v", err)
	}

	binlogIndexBytes, err := a.fileManager.ReadStateFile(binaryLogIndex)
	if err != nil {
		return nil, fmt.Errorf("error reading binary log index: %v", err)
	}

	var binlogs []string
	fileScanner := bufio.NewScanner(bytes.NewReader(binlogIndexBytes))
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		binlog := path.Base(fileScanner.Text())
		binlogs = append(binlogs, binlog)
	}
	return binlogs, nil
}

func (a *Archiver) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return a.client.Status().Patch(ctx, mariadb, patch)
}

func getS3Client(pitr *mariadbv1alpha1.PointInTimeRecovery) (*minio.Client, error) {
	s3 := pitr.Spec.S3
	tls := ptr.Deref(s3.TLS, mariadbv1alpha1.TLSS3{})

	clientOpts := []mariadbminio.MinioOpt{
		mariadbminio.WithTLS(tls.Enabled),
		// TODO: mount TLS certs
		// mariadbminio.WithCACertPath(opts.CACertPath),
		mariadbminio.WithRegion(s3.Region),
	}
	client, err := mariadbminio.NewMinioClient(s3.Endpoint, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 client: %v", err)
	}
	return client, nil
}

func shouldResetArchivedBinlog(binlogs []string, lastArchivedBinlog string,
	logger logr.Logger) (bool, error) {
	var errBundle *multierror.Error
	prefix, err := BinlogPrefix(binlogs[0])
	errBundle = multierror.Append(errBundle, err)
	archivedPrefix, err := BinlogPrefix(lastArchivedBinlog)
	errBundle = multierror.Append(errBundle, err)

	lastNum, err := BinlogNum(binlogs[len(binlogs)-1])
	errBundle = multierror.Append(errBundle, err)
	archivedNum, err := BinlogNum(lastArchivedBinlog)
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return false, err
	}
	if prefix != archivedPrefix || *lastNum < *archivedNum {
		logger.Info(
			"Resetting last archived binary log",
			"prefix", prefix,
			"archived-prefix", archivedPrefix,
			"last-num", lastNum,
			"archived-num", archivedNum,
		)
		return true, nil
	}
	return false, nil
}
