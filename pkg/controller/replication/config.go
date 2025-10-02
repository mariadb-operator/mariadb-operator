package replication

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	env "github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/version"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	replUser     = "repl"
	replUserHost = "%"
)

type ReplicationConfigClient struct {
	client.Client
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
	env              *env.OperatorEnv
}

func NewReplicationConfigClient(client client.Client, builder *builder.Builder, secretReconciler *secret.SecretReconciler,
	env *env.OperatorEnv) *ReplicationConfigClient {
	return &ReplicationConfigClient{
		Client:           client,
		builder:          builder,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
		env:              env,
	}
}

func (r *ReplicationConfigClient) ConfigurePrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	podIndex int) error {
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := client.ResetAllSlaves(ctx); err != nil {
		return fmt.Errorf("error resetting slave: %v", err)
	}
	if err := client.ResetGtidSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := client.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling read_only: %v", err)
	}
	if err := r.reconcilePrimarySql(ctx, mariadb, client); err != nil {
		return fmt.Errorf("error reconciling primary SQL: %v", err)
	}
	return nil
}

type ConfigureReplicaOpts struct {
	GtidSlavePos      *string
	ResetGtidSlavePos bool
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

func (r *ReplicationConfigClient) ConfigureReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	replicaPodIndex, primaryPodIndex int, replicaOpts ...ConfigureReplicaOpt) error {
	opts := ConfigureReplicaOpts{}
	for _, setOpt := range replicaOpts {
		setOpt(&opts)
	}

	if err := client.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := client.StopAllSlaves(ctx); err != nil {
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
	if err := r.changeMaster(ctx, mariadb, client, primaryPodIndex); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationConfigClient) changeMaster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	primaryPodIndex int) error {
	replPasswordRef := newReplPasswordRef(mariadb)

	password, err := r.refResolver.SecretKeyRef(ctx, replPasswordRef.SecretKeySelector, mariadb.Namespace)
	if err != nil {
		return fmt.Errorf("error getting replication password: %v", err)
	}

	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})

	gtid := ptr.Deref(replication.Replica.Gtid, mariadbv1alpha1.GtidCurrentPos)
	gtidString, err := gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}

	changeMasterHostOpt, err := r.getChangeMasterHost(ctx, mariadb, primaryPodIndex)
	if err != nil {
		return fmt.Errorf("error getting host option: %v", err)
	}

	changeMasterOpts := []sql.ChangeMasterOpt{
		changeMasterHostOpt,
		sql.WithChangeMasterPort(mariadb.Spec.Port),
		sql.WithChangeMasterCredentials(replUser, password),
		sql.WithChangeMasterGtid(gtidString),
		sql.WithChangeMasterRetries(*replication.Replica.ConnectionRetries),
	}
	if mariadb.IsTLSEnabled() {
		changeMasterOpts = append(changeMasterOpts, sql.WithChangeMasterSSL(
			builderpki.ClientCertPath,
			builderpki.ClientKeyPath,
			builderpki.CACertPath,
		))
	}
	if err := client.ChangeMaster(ctx, changeMasterOpts...); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	return nil
}

func (r *ReplicationConfigClient) getChangeMasterHost(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	primaryPodIndex int) (sql.ChangeMasterOpt, error) {
	logger := log.FromContext(ctx).
		WithName("replication-config").
		WithValues("image", mariadb.Spec.Image).
		V(1)
	vOpts := []version.Option{
		version.WithLogger(logger),
	}
	if r.env != nil && r.env.MariadbDefaultVersion != "" {
		vOpts = append(vOpts, version.WithDefaultVersion(r.env.MariadbDefaultVersion))
	}
	v, err := version.NewVersion(mariadb.Spec.Image, vOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating version: %v", err)
	}

	isCompatibleVersion, err := v.GreaterThanOrEqual("10.6")
	if err != nil {
		return nil, fmt.Errorf("error comparing version: %v", err)
	}

	if isCompatibleVersion {
		return sql.WithChangeMasterHost(
			statefulset.PodFQDNWithService(
				mariadb.ObjectMeta,
				primaryPodIndex,
				mariadb.InternalServiceKey().Name,
			),
		), nil
	}
	return sql.WithChangeMasterHost(
		// MariaDB 10.5 has a limitation of 60 characters in this host.
		statefulset.PodShortFQDNWithService(
			mariadb.ObjectMeta,
			primaryPodIndex,
			mariadb.InternalServiceKey().Name,
		),
	), nil
}

func (r *ReplicationConfigClient) reconcilePrimarySql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client) error {
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

func (r *ReplicationConfigClient) reconcileUserSql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	opts *userSqlOpts) error {
	replPasswordRef := newReplPasswordRef(mariadb)
	var replPassword string

	req := secret.PasswordRequest{
		Owner:    mariadb,
		Metadata: mariadb.Spec.InheritMetadata,
		Key: types.NamespacedName{
			Name:      replPasswordRef.Name,
			Namespace: mariadb.Namespace,
		},
		SecretKey: replPasswordRef.Key,
		Generate:  replPasswordRef.Generate,
	}
	replPassword, err := r.secretReconciler.ReconcilePassword(ctx, req)
	if err != nil {
		return fmt.Errorf("error reconciling replication password: %v", err)
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

func NewReplicationConfig(env *env.PodEnvironment) ([]byte, error) {
	replEnabled, err := env.IsReplEnabled()
	if err != nil {
		return nil, fmt.Errorf("error checking if replication is enabled: %v", err)
	}
	if !replEnabled {
		return nil, errors.New("replication must be enabled")
	}
	gtidStrictMode, err := env.ReplGtidStrictMode()
	if err != nil {
		return nil, fmt.Errorf("error getting GTID strict mode: %v", err)
	}
	masterTimeout, err := env.ReplMasterTimeout()
	if err != nil {
		return nil, fmt.Errorf("error getting master timeout: %v", err)
	}
	serverId, err := serverId(env.PodName)
	if err != nil {
		return nil, fmt.Errorf("error getting server ID: %v", err)
	}
	syncBinlog, err := env.ReplSyncBinlog()
	if err != nil {
		return nil, fmt.Errorf("error getting master sync binlog: %v", err)
	}

	// To facilitate switchover/failover and avoid clashing with MaxScale, this configuration allows any Pod to act either as a primary or a replica.
	// See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/semisynchronous-replication#enabling-semisynchronous-replication
	tpl := createTpl("replication", `[mariadb]
log_bin
log_basename={{.LogName }}
{{- with .GtidStrictMode }}
gtid_strict_mode
{{- end }}
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
{{- with .MasterTimeout }}
rpl_semi_sync_master_timeout={{ . }}
{{- end }}
{{- with .MasterWaitPoint }}
rpl_semi_sync_master_wait_point={{ . }}
{{- end }}
server_id={{ .ServerId }}
{{- with .SyncBinlog }}
sync_binlog={{ . }}
{{- end }}
`)
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, struct {
		LogName         string
		GtidStrictMode  bool
		MasterTimeout   *int64
		MasterWaitPoint string
		SyncBinlog      *int
		ServerId        int
	}{
		LogName:         env.MariadbName,
		GtidStrictMode:  gtidStrictMode,
		MasterTimeout:   masterTimeout,
		MasterWaitPoint: env.MariaDBReplMasterWaitPoint,
		ServerId:        serverId,
		SyncBinlog:      syncBinlog,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newReplPasswordRef(mariadb *mariadbv1alpha1.MariaDB) mariadbv1alpha1.GeneratedSecretKeyRef {
	replication := ptr.Deref(mariadb.Spec.Replication, mariadbv1alpha1.Replication{})
	if replication.Enabled && replication.Replica.ReplPasswordSecretKeyRef != nil {
		return *replication.Replica.ReplPasswordSecretKeyRef
	}
	return mariadbv1alpha1.GeneratedSecretKeyRef{
		SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: fmt.Sprintf("repl-password-%s", mariadb.Name),
			},
			Key: "password",
		},
		Generate: true,
	}
}

func serverId(podName string) (int, error) {
	podIndex, err := statefulset.PodIndex(podName)
	if err != nil {
		return 0, fmt.Errorf("error getting Pod index: %v", err)
	}
	return 10 + *podIndex, nil
}

func formatAccountName(username, host string) string {
	return fmt.Sprintf("'%s'@'%s'", username, host)
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
