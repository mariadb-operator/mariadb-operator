package binlog

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/minio/minio-go/v7"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	a.logger.Info("Starting binlog archiver")

	mdb, err := a.getMariaDB(ctx)
	if err != nil {
		return err
	}
	pitr, err := a.getPointInTimeRecovery(ctx, mdb)
	if err != nil {
		return err
	}
	// TODO: mount TLS certs and credentials
	_, err = getS3Client(pitr)
	if err != nil {
		return err
	}

	return nil
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
