package replication

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
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
	ConfigureReplica(ctx context.Context, client *sql.Client, primaryPodIndex int, replicaOpts ...ConfigureReplicaOpt) error
}

type TopologyManager struct {
	client.Client
	refResolver *refresolver.RefResolver
}

func NewTopologyManager(client client.Client) *TopologyManager {
	return &TopologyManager{
		Client:      client,
		refResolver: refresolver.New(client),
	}
}

func (t *TopologyManager) TopologyForMariaDB(mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) Topology {
	if mariadb.IsMultiClusterEnabled() && mariadb.IsReplicationEnabled() {
		logger.V(1).Info("Configuring multi-cluster replication topology")
		multiClusterLogger := logger.WithName("multi-cluster")

		return newMultiClusterTopology(
			mariadb,
			newSingleClusterTopology(
				mariadb,
				t.Client,
				t.refResolver,
				multiClusterLogger,
			),
			t.Client,
			t.refResolver,
			multiClusterLogger,
		)
	}
	// TODO: multi-cluster with Galera

	logger.V(1).Info("Configuring single-cluster replication topology")
	return newSingleClusterTopology(
		mariadb,
		t.Client,
		t.refResolver,
		logger.WithName("single-cluster"),
	)
}

type singleClusterTopology struct {
	client.Client
	mariadb     *mariadbv1alpha1.MariaDB
	refResolver *refresolver.RefResolver
	logger      logr.Logger
}

func newSingleClusterTopology(mariadb *mariadbv1alpha1.MariaDB, client client.Client, refResolver *refresolver.RefResolver,
	logger logr.Logger) *singleClusterTopology {
	return &singleClusterTopology{
		Client:      client,
		mariadb:     mariadb,
		refResolver: refResolver,
		logger:      logger,
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
		if err := client.ResetSlave(ctx); err != nil {
			return fmt.Errorf("error resetting slave: %v", err)
		}
		if err := client.ResetGtidSlavePos(ctx); err != nil {
			// This error could happen when log_slave_updates=0N (multi-cluster, PITR),
			// when the replica to be promoted already has binary logs.
			// If returned, this error will completely block switchover/failover operations.
			// Error 1948 (HY000): Specified value for @@gtid_slave_pos contains no value for
			// replication domain 0. This conflicts with the binary log which contains GTID
			// 0-11-1176. If MASTER_GTID_POS=CURRENT_POS is used, the binlog position will
			// override the new value of @@gtid_slave_pos'
			if sql.IsGtidSlavePosNoValueForDomain(err) {
				return nil
			}
			return fmt.Errorf("error resetting slave position: %v", err)
		}
	}
	if err := client.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling read_only: %v", err)
	}
	if err := r.reconcilePrimarySql(ctx, client); err != nil {
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
			return fmt.Errorf("error setting slave position %s: %v", *opts.GtidSlavePos, err)
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

func (r *singleClusterTopology) reconcilePrimarySql(ctx context.Context, client *sql.Client) error {
	r.logger.V(1).Info("Reconciling primary SQL")

	opts := userSqlOpts{
		username:   replUser,
		host:       replUserHost,
		privileges: []string{"REPLICATION REPLICA"},
	}
	if err := r.reconcileUserSql(ctx, client, &opts); err != nil {
		return fmt.Errorf("error reconciling '%s' SQL user: %v", replUser, err)
	}
	return nil
}

type userSqlOpts struct {
	username   string
	host       string
	privileges []string
}

func (r *singleClusterTopology) reconcileUserSql(ctx context.Context, client *sql.Client,
	opts *userSqlOpts) error {
	r.logger.V(1).Info("Reconciling user SQL")

	replication := ptr.Deref(r.mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Replica.ReplPasswordSecretKeyRef == nil {
		return errors.New("'spec.replication.replica.replPasswordSecretKeyRef` must not be nil'")
	}

	replPassword, err := r.refResolver.SecretKeyRef(ctx, replication.Replica.ReplPasswordSecretKeyRef.SecretKeySelector, r.mariadb.Namespace)
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

type multiClusterTopology struct {
	client.Client
	mariadb       *mariadbv1alpha1.MariaDB
	singleCluster *singleClusterTopology
	refResolver   *refresolver.RefResolver
	logger        logr.Logger
}

func newMultiClusterTopology(mariadb *mariadbv1alpha1.MariaDB, singleCluster *singleClusterTopology,
	client client.Client, refResolver *refresolver.RefResolver, logger logr.Logger) *multiClusterTopology {
	return &multiClusterTopology{
		Client:        client,
		mariadb:       mariadb,
		singleCluster: singleCluster,
		refResolver:   refResolver,
		logger:        logger,
	}
}

func (m *multiClusterTopology) ConfigurePrimary(ctx context.Context, client *sql.Client) error {
	if m.mariadb.IsMultiClusterPrimary() {
		return m.singleCluster.ConfigurePrimary(ctx, client)
	}
	return m.configurePrimaryReplica(ctx, client)
}

func (m *multiClusterTopology) ConfigureReplica(ctx context.Context, client *sql.Client, primaryPodIndex int,
	replicaOpts ...ConfigureReplicaOpt) error {
	opts := slices.Clone(replicaOpts)
	// keep binary logs in replicas: when promoted to new primary, they will have binary logs to dump to the replica cluster
	opts = append(opts, WithResetMaster(false))

	if m.mariadb.IsMultiClusterPrimary() {
		return m.singleCluster.ConfigureReplica(ctx, client, primaryPodIndex, opts...)
	}
	return m.configureSecondaryReplica(ctx, client, primaryPodIndex, opts...)
}

func (m *multiClusterTopology) configurePrimaryReplica(ctx context.Context, client *sql.Client) error {
	m.logger.Info("Configuring primary replica")

	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping all slaves: %v", err)
	}
	// reset local replica
	if err := client.ResetSlave(ctx); err != nil {
		return fmt.Errorf("error resetting local slave: %v", err)
	}

	if err := client.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling read_only: %v", err)
	}
	if err := m.singleCluster.reconcilePrimarySql(ctx, client); err != nil {
		return fmt.Errorf("error reconciling primary SQL: %v", err)
	}

	if err := m.changeMasterPrimaryInPrimaryReplica(ctx, client); err != nil {
		return fmt.Errorf("error changing master in primary replica: %v", err)
	}
	// start remote replica
	if err := client.StartSlave(ctx, sql.WithConnectionName(MultiClusterReplicaConnectionName)); err != nil {
		return fmt.Errorf("error starting primary replica slave: %v", err)
	}
	return nil
}

func (m *multiClusterTopology) changeMasterPrimaryInPrimaryReplica(ctx context.Context, client *sql.Client) error {
	member := m.mariadb.GetMultiClusterPrimary()
	if member == nil {
		return errors.New("unable to find multi-cluster primary member")
	}

	externalMariaDBRef, err := m.mariadb.Spec.MultiCluster.GetExternalMariaDBRefForMember(*member)
	if err != nil {
		return fmt.Errorf("error getting ExternalMariaDB reference for member %s: %v", *member, err)
	}
	externalMariaDB, err := m.refResolver.ExternalMariaDB(ctx, externalMariaDBRef, m.mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting ExternalMariaDB: %v", err)
	}

	password, err := m.refResolver.SecretKeyRef(ctx, *externalMariaDB.Spec.PasswordSecretKeyRef, externalMariaDB.Namespace)
	if err != nil {
		return fmt.Errorf("error getting ExternalMariaDB password: %v", err)
	}

	gtidString, err := mariadbv1alpha1.GtidSlavePos.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID position: %v", err)
	}

	opts := []sql.ChangeMasterOpt{
		sql.WithChangeMasterConnectionName(MultiClusterReplicaConnectionName),
		sql.WithChangeMasterHost(externalMariaDB.GetHost()),
		sql.WithChangeMasterPort(externalMariaDB.GetPort()),
		sql.WithChangeMasterCredentials(externalMariaDB.GetSUName(), password),
		sql.WithChangeMasterGtid(gtidString),
	}
	if externalMariaDB.IsTLSEnabled() {
		opts = append(opts, sql.WithChangeMasterSSL(
			builderpki.ClientCertPath,
			builderpki.ClientKeyPath,
			builderpki.CACertPath,
		))
	}
	if err := client.ChangeMaster(ctx, opts...); err != nil {
		return fmt.Errorf("error executing CHANGE MASTER in primary replica: %v", err)
	}
	return nil
}

func (m *multiClusterTopology) configureSecondaryReplica(ctx context.Context, client *sql.Client, primaryPodIndex int,
	replicaOpts ...ConfigureReplicaOpt) error {
	m.logger.Info("Configuring secondary replica")

	opts := ConfigureReplicaOpts{}
	for _, setOpt := range replicaOpts {
		setOpt(&opts)
	}

	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping all replicas: %v", err)
	}
	if err := client.ResetSlave(
		ctx,
		sql.WithConnectionName(MultiClusterReplicaConnectionName),
	); err != nil && !sql.IsConnectionNotExists(err) {
		return fmt.Errorf("error resetting remote replica: %v", err)
	}
	if err := client.ResetSlave(ctx); err != nil {
		return fmt.Errorf("error resetting local replica: %v", err)
	}

	if opts.GtidSlavePos != nil {
		if err := client.SetGtidSlavePos(ctx, *opts.GtidSlavePos); err != nil {
			return fmt.Errorf("error setting slave position %s: %v", *opts.GtidSlavePos, err)
		}
	} else if opts.ResetGtidSlavePos {
		if err := client.ResetGtidSlavePos(ctx); err != nil {
			return fmt.Errorf("error resetting slave position: %v", err)
		}
	}
	if err := client.EnableReadOnly(ctx); err != nil {
		return fmt.Errorf("error enabling read_only: %v", err)
	}
	if err := m.singleCluster.changeMaster(ctx, m.mariadb, client, primaryPodIndex, opts.ChangeMasterOpts...); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return fmt.Errorf("error starting local replica: %v", err)
	}
	return nil
}
