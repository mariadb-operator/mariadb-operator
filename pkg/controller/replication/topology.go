package replication

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	MultiClusterReplicaConnectionName = "replica"

	replUser     = "repl"
	replUserHost = "%"
)

type ConfigureReplicaOpts struct {
	GtidSlavePos      *string
	ResetGtidSlavePos bool
	ChangeMasterOpts  []sql.ChangeMasterOpt
	ResetMaster       bool
}

type ConfigureReplicaOpt func(*ConfigureReplicaOpts)

func WithGtidSlavePos(gtid string) ConfigureReplicaOpt {
	return func(cro *ConfigureReplicaOpts) {
		cro.GtidSlavePos = &gtid
	}
}

func WithResetGtidSlavePos() ConfigureReplicaOpt {
	return func(cro *ConfigureReplicaOpts) {
		cro.ResetGtidSlavePos = true
	}
}

func WithChangeMasterOpts(opts ...sql.ChangeMasterOpt) ConfigureReplicaOpt {
	return func(cro *ConfigureReplicaOpts) {
		cro.ChangeMasterOpts = opts
	}
}

func WithResetMaster(resetMaster bool) ConfigureReplicaOpt {
	return func(cro *ConfigureReplicaOpts) {
		cro.ResetMaster = resetMaster
	}
}

type Topology interface {
	ConfigurePrimary(ctx context.Context, client *sql.Client) error
	ConfigureReplica(ctx context.Context, client *sql.Client, primaryPodIndex int,
		replicaOpts ...ConfigureReplicaOpt) error
}

type TopologyManager struct {
	client.Client
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
}

func NewTopologyManager(client client.Client, builder *builder.Builder,
	secretReconciler *secret.SecretReconciler) *TopologyManager {
	return &TopologyManager{
		Client:           client,
		builder:          builder,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
	}
}

func (t *TopologyManager) TopologyForMariaDB(mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) Topology {
	if mariadb.IsMultiClusterEnabled() && mariadb.IsReplicationEnabled() {
		logger.V(1).Info("Configuring multi-cluster replication topology")
		return NewMultiClusterTopology() // TODO: implement
	}
	// TODO: multi-cluster with Galera

	logger.V(1).Info("Configuring single-cluster replication topology")
	return newSingleClusterTopology(
		mariadb,
		t.Client,
		t.builder,
		t.refResolver,
		t.secretReconciler,
		logger.WithName("single-cluster"),
	)
}

type singleClusterTopology struct {
	client.Client
	mariadb          *mariadbv1alpha1.MariaDB
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
	logger           logr.Logger
}

func newSingleClusterTopology(mariadb *mariadbv1alpha1.MariaDB, client client.Client, builder *builder.Builder,
	refResolver *refresolver.RefResolver, secretReconciler *secret.SecretReconciler, logger logr.Logger) Topology {
	return &singleClusterTopology{
		Client:           client,
		mariadb:          mariadb,
		builder:          builder,
		refResolver:      refResolver,
		secretReconciler: secretReconciler,
		logger:           logger,
	}
}

func (r *singleClusterTopology) ConfigurePrimary(ctx context.Context, client *sql.Client) error {
	r.logger.Info("Configuring primary")

	isReplica, err := client.IsReplicationReplica(ctx)
	if err != nil {
		return fmt.Errorf("error checking replica: %v", err)
	}
	if isReplica {
		if err := client.StopSlave(ctx); err != nil {
			return fmt.Errorf("error stopping slaves: %v", err)
		}
		if err := client.ResetSlaveAll(ctx); err != nil {
			return fmt.Errorf("error resetting slave: %v", err)
		}
		if err := client.ResetGtidSlavePos(ctx); err != nil {
			return fmt.Errorf("error resetting slave position: %v", err)
		}
	}
	if err := client.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling read_only: %v", err)
	}
	if err := r.reconcilePrimarySql(ctx, r.mariadb, client); err != nil {
		return fmt.Errorf("error reconciling primary SQL: %v", err)
	}
	return nil
}

func (r *singleClusterTopology) ConfigureReplica(ctx context.Context, client *sql.Client,
	primaryPodIndex int, replicaOpts ...ConfigureReplicaOpt) error {
	r.logger.Info("Configuring replica")

	opts := ConfigureReplicaOpts{
		ResetMaster: true,
	}
	for _, setOpt := range replicaOpts {
		setOpt(&opts)
	}

	if opts.ResetMaster {
		if err := client.ResetMaster(ctx); err != nil {
			return fmt.Errorf("error resetting master: %v", err)
		}
	}
	if err := client.StopSlave(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if opts.GtidSlavePos != nil {
		if err := client.SetGtidSlavePos(ctx, *opts.GtidSlavePos); err != nil {
			return fmt.Errorf("error setting slave position \"%s\": %v", *opts.GtidSlavePos, err)
		}
	} else if opts.ResetGtidSlavePos {
		if err := client.ResetGtidSlavePos(ctx); err != nil {
			return fmt.Errorf("error resetting slave position: %v", err)
		}
	}
	if err := client.EnableReadOnly(ctx); err != nil {
		return fmt.Errorf("error enabling read_only: %v", err)
	}
	if err := r.changeMaster(ctx, r.mariadb, client, primaryPodIndex, opts.ChangeMasterOpts...); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *singleClusterTopology) changeMaster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	primaryPodIndex int, opts ...sql.ChangeMasterOpt) error {
	r.logger.V(1).Info("Changing master")

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Replica.ReplPasswordSecretKeyRef == nil {
		return errors.New("'spec.replication.replica.replPasswordSecretKeyRef` must not be nil'")
	}

	password, err := r.refResolver.SecretKeyRef(ctx, replication.Replica.ReplPasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting replication password: %v", err)
	}

	gtid := ptr.Deref(replication.Replica.Gtid, mariadbv1alpha1.GtidCurrentPos)
	gtidString, err := gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting change master GTID: %v", err)
	}

	changeMasterOpts := []sql.ChangeMasterOpt{
		sql.WithChangeMasterHost(
			statefulset.PodFQDNWithService(
				mariadb.ObjectMeta,
				primaryPodIndex,
				mariadb.InternalServiceKey().Name,
			),
		),
		sql.WithChangeMasterPort(mariadb.Spec.Port),
		sql.WithChangeMasterCredentials(replUser, password),
		sql.WithChangeMasterGtid(gtidString),
	}
	if mariadb.IsTLSEnabled() {
		changeMasterOpts = append(changeMasterOpts, sql.WithChangeMasterSSL(
			builderpki.ClientCertPath,
			builderpki.ClientKeyPath,
			builderpki.CACertPath,
		))
	}

	if retries := ptr.Deref(replication.Replica.ConnectionRetrySeconds, -1); retries != -1 {
		changeMasterOpts = append(changeMasterOpts, sql.WithChangeMasterRetries(*replication.Replica.ConnectionRetrySeconds))
	}

	changeMasterOpts = append(changeMasterOpts, opts...)

	if err := client.ChangeMaster(ctx, changeMasterOpts...); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	return nil
}

func (r *singleClusterTopology) reconcilePrimarySql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client) error {
	r.logger.V(1).Info("Reconciling primary SQL")

	opts := userSqlOpts{
		username:   replUser,
		host:       replUserHost,
		privileges: []string{"REPLICATION REPLICA"},
	}
	if err := r.reconcileUserSql(ctx, mariadb, client, &opts); err != nil {
		return fmt.Errorf("error reconciling '%s' SQL user: %v", replUser, err)
	}
	return nil
}

type userSqlOpts struct {
	username   string
	host       string
	privileges []string
}

func (r *singleClusterTopology) reconcileUserSql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	opts *userSqlOpts) error {
	r.logger.V(1).Info("Reconciling user SQL")

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Replica.ReplPasswordSecretKeyRef == nil {
		return errors.New("'spec.replication.replica.replPasswordSecretKeyRef` must not be nil'")
	}

	replPassword, err := r.refResolver.SecretKeyRef(ctx, replication.Replica.ReplPasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting repl password: %v", err)
	}
	accountName := formatAccountName(opts.username, opts.host)

	exists, err := client.UserExists(ctx, opts.username, opts.host)
	if err != nil {
		return fmt.Errorf("error checking if replication user exists: %v", err)
	}
	if exists {
		if err := client.AlterUser(ctx, accountName, sql.WithIdentifiedBy(replPassword)); err != nil {
			return fmt.Errorf("error altering replication user: %v", err)
		}
	} else {
		if err := client.CreateUser(ctx, accountName, sql.WithIdentifiedBy(replPassword)); err != nil {
			return fmt.Errorf("error creating replication user: %v", err)
		}
	}
	if err := client.Grant(
		ctx,
		opts.privileges,
		"*",
		"*",
		accountName,
	); err != nil {
		return fmt.Errorf("error creating grant: %v", err)
	}
	return nil
}

type MultiClusterTopology struct{}

func NewMultiClusterTopology() Topology {
	return &MultiClusterTopology{}
}

// ConfigurePrimary implements [Topology].
func (m *MultiClusterTopology) ConfigurePrimary(ctx context.Context, client *sql.Client) error {
	panic("unimplemented")
}

// ConfigureReplica implements [Topology].
func (m *MultiClusterTopology) ConfigureReplica(ctx context.Context, client *sql.Client, primaryPodIndex int,
	replicaOpts ...ConfigureReplicaOpt) error {
	panic("unimplemented")
}
