package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	passwordSecretKey = "password"
	replUser          = "repl"
	connectionName    = "mariadb-operator"
)

type ReplicationConfig struct {
	client.Client
	mariadb       *mariadbv1alpha1.MariaDB
	mariadbClient *mariadb.Client
	builder       *builder.Builder
	refResolver   *refresolver.RefResolver
}

func NewReplicationConfig(mariadb *mariadbv1alpha1.MariaDB, mariadbClient *mariadb.Client,
	client client.Client, builder *builder.Builder) *ReplicationConfig {
	return &ReplicationConfig{
		Client:        client,
		mariadb:       mariadb,
		mariadbClient: mariadbClient,
		builder:       builder,
		refResolver:   refresolver.New(client),
	}
}

func (r *ReplicationConfig) ConfigurePrimary(ctx context.Context, podIndex int) error {
	if err := r.mariadbClient.UnlockTables(ctx); err != nil {
		return fmt.Errorf("error unlocking tables: %v", err)
	}
	if err := r.mariadbClient.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := r.mariadbClient.ResetAllSlaves(ctx); err != nil {
		return fmt.Errorf("error resetting slave: %v", err)
	}
	if err := r.mariadbClient.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := r.mariadbClient.SetGlobalVar(ctx, "read_only", "0"); err != nil {
		return fmt.Errorf("error setting read_only=0: %v", err)
	}
	if err := r.configurePrimaryVars(ctx, podIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := r.reconcilePrimarySql(ctx); err != nil {
		return fmt.Errorf("error reconciling primary SQL: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) ConfigureReplica(ctx context.Context, replicaPodIndex, primaryPodIndex int) error {
	if err := r.mariadbClient.UnlockTables(ctx); err != nil {
		return fmt.Errorf("error unlocking tables: %v", err)
	}
	if err := r.mariadbClient.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := r.mariadbClient.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := r.mariadbClient.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := r.mariadbClient.SetGlobalVar(ctx, "read_only", "1"); err != nil {
		return fmt.Errorf("error setting read_only=1: %v", err)
	}
	if err := r.configureReplicaVars(ctx, r.mariadb, r.mariadbClient, replicaPodIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := r.changeMaster(ctx, r.mariadb, primaryPodIndex); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := r.mariadbClient.StartSlave(ctx, connectionName); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configurePrimaryVars(ctx context.Context, primaryPodIndex int) error {
	kv := map[string]string{
		"rpl_semi_sync_master_enabled": "ON",
		"rpl_semi_sync_master_timeout": func() string {
			return fmt.Sprint(r.mariadb.Spec.Replication.Replica.ConnectionTimeoutOrDefault().Milliseconds())
		}(),
		"rpl_semi_sync_slave_enabled": "OFF",
		"server_id":                   serverId(primaryPodIndex),
	}
	if r.mariadb.Spec.Replication.Replica.WaitPoint != nil {
		waitPoint, err := r.mariadb.Spec.Replication.Replica.WaitPoint.MariaDBFormat()
		if err != nil {
			return fmt.Errorf("error getting wait point: %v", err)
		}
		kv["rpl_semi_sync_master_wait_point"] = waitPoint
	}
	if err := r.mariadbClient.SetGlobalVars(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configureReplicaVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	client *mariadb.Client, ordinal int) error {
	kv := map[string]string{
		"rpl_semi_sync_master_enabled": "OFF",
		"rpl_semi_sync_slave_enabled":  "ON",
		"server_id":                    serverId(ordinal),
	}
	if err := client.SetGlobalVars(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) changeMaster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, primaryPodIndex int) error {
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
		return fmt.Errorf("error getting replication password Secret: %v", err)
	}
	changeMasterOpts := &mariadbclient.ChangeMasterOpts{
		Connection: connectionName,
		Host: statefulset.PodFQDN(
			mariadb.ObjectMeta,
			primaryPodIndex,
		),
		User:     replUser,
		Password: string(replSecret.Data[passwordSecretKey]),
		Gtid:     "current_pos",
		Retries:  mariadb.Spec.Replication.Replica.ConnectionRetries,
	}
	if err := r.mariadbClient.ChangeMaster(ctx, changeMasterOpts); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) reconcilePrimarySql(ctx context.Context) error {
	if r.mariadb.Spec.Username != nil && r.mariadb.Spec.PasswordSecretKeyRef != nil {
		password, err := r.refResolver.SecretKeyRef(ctx, *r.mariadb.Spec.PasswordSecretKeyRef, r.mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting password: %v", err)
		}
		userOpts := mariadbclient.CreateUserOpts{
			IdentifiedBy: password,
		}
		if err := r.mariadbClient.CreateUser(ctx, *r.mariadb.Spec.Username, userOpts); err != nil {
			return fmt.Errorf("error creating user: %v", err)
		}

		grantOpts := mariadbclient.GrantOpts{
			Privileges:  []string{"ALL PRIVILEGES"},
			Database:    "*",
			Table:       "*",
			Username:    *r.mariadb.Spec.Username,
			GrantOption: false,
		}
		if err := r.mariadbClient.Grant(ctx, grantOpts); err != nil {
			return fmt.Errorf("error creating grant: %v", err)
		}
	}

	if r.mariadb.Spec.Database != nil {
		databaseOpts := mariadbclient.DatabaseOpts{
			CharacterSet: "utf8",
			Collate:      "utf8_general_ci",
		}
		if err := r.mariadbClient.CreateDatabase(ctx, *r.mariadb.Spec.Database, databaseOpts); err != nil {
			return fmt.Errorf("error creating database: %v", err)
		}
	}

	password, err := r.reconcileReplPasswordSecret(ctx)
	if err != nil {
		return fmt.Errorf("error reconciling replication passsword: %v", err)
	}
	userOpts := mariadbclient.CreateUserOpts{
		IdentifiedBy: password,
	}
	if err := r.mariadbClient.CreateUser(ctx, replUser, userOpts); err != nil {
		return fmt.Errorf("error creating replication user: %v", err)
	}
	grantOpts := mariadbclient.GrantOpts{
		Privileges:  []string{"REPLICATION REPLICA"},
		Database:    "*",
		Table:       "*",
		Username:    replUser,
		GrantOption: false,
	}
	if err := r.mariadbClient.Grant(ctx, grantOpts); err != nil {
		return fmt.Errorf("error creating grant: %v", err)
	}
	if err := r.mariadbClient.FlushPrivileges(ctx); err != nil {
		return fmt.Errorf("error flushing privileges: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) reconcileReplPasswordSecret(ctx context.Context) (string, error) {
	var existingSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(r.mariadb), &existingSecret); err == nil {
		return "", nil
	}
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return "", fmt.Errorf("error generating replication password: %v", err)
	}

	opts := builder.SecretOpts{
		Key: replPasswordKey(r.mariadb),
		Data: map[string][]byte{
			passwordSecretKey: []byte(password),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(r.mariadb).Build(),
	}
	secret, err := r.builder.BuildSecret(opts, r.mariadb)
	if err != nil {
		return "", fmt.Errorf("error building replication password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return "", fmt.Errorf("error creating replication password Secret: %v", err)
	}

	return password, nil
}

func serverId(index int) string {
	return fmt.Sprint(10 + index)
}
