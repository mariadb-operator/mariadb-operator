package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controllers/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	replUser       = "repl"
	connectionName = "mariadb-operator"
)

type ReplicationConfig struct {
	client.Client
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
}

func NewReplicationConfig(client client.Client, builder *builder.Builder, secretReconciler *secret.SecretReconciler) *ReplicationConfig {
	return &ReplicationConfig{
		Client:           client,
		builder:          builder,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
	}
}

func (r *ReplicationConfig) ConfigurePrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *mariadbclient.Client,
	podIndex int) error {
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := client.ResetAllSlaves(ctx); err != nil {
		return fmt.Errorf("error resetting slave: %v", err)
	}
	if err := client.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := client.DisableReadOnly(ctx); err != nil {
		return fmt.Errorf("error disabling read_only: %v", err)
	}
	if err := r.reconcilePrimarySql(ctx, mariadb, client); err != nil {
		return fmt.Errorf("error reconciling primary SQL: %v", err)
	}
	if err := r.configurePrimaryVars(ctx, mariadb, client, podIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) ConfigureReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *mariadbclient.Client,
	replicaPodIndex, primaryPodIndex int) error {
	if err := client.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := client.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave position: %v", err)
	}
	if err := client.EnableReadOnly(ctx); err != nil {
		return fmt.Errorf("error enabling read_only: %v", err)
	}
	if err := r.configureReplicaVars(ctx, mariadb, client, replicaPodIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := r.changeMaster(ctx, mariadb, client, primaryPodIndex); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx, connectionName); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configurePrimaryVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *mariadbclient.Client,
	primaryPodIndex int) error {
	kv := map[string]string{
		"sync_binlog":                  binaryFromBool(mariadb.Spec.Replication.SyncBinlog),
		"rpl_semi_sync_master_enabled": "ON",
		"rpl_semi_sync_master_timeout": func() string {
			return fmt.Sprint(mariadb.Spec.Replication.Replica.ConnectionTimeoutOrDefault().Milliseconds())
		}(),
		"rpl_semi_sync_slave_enabled": "OFF",
		"server_id":                   serverId(primaryPodIndex),
	}
	if mariadb.Spec.Replication.Replica.WaitPoint != nil {
		waitPoint, err := mariadb.Spec.Replication.Replica.WaitPoint.MariaDBFormat()
		if err != nil {
			return fmt.Errorf("error getting wait point: %v", err)
		}
		kv["rpl_semi_sync_master_wait_point"] = waitPoint
	}
	if err := client.SetSystemVariables(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configureReplicaVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	client *mariadbclient.Client, ordinal int) error {
	kv := map[string]string{
		"sync_binlog":                  binaryFromBool(mariadb.Spec.Replication.SyncBinlog),
		"rpl_semi_sync_master_enabled": "OFF",
		"rpl_semi_sync_slave_enabled":  "ON",
		"server_id":                    serverId(ordinal),
	}
	if err := client.SetSystemVariables(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) changeMaster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *mariadbclient.Client,
	primaryPodIndex int) error {
	replPasswordRef := newReplPasswordRef(mariadb)
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordRef.NamespacedName, &replSecret); err != nil {
		return fmt.Errorf("error getting replication password Secret: %v", err)
	}

	gtid := mariadbv1alpha1.GtidCurrentPos
	if mariadb.Spec.Replication.Replica.Gtid != nil {
		gtid = *mariadb.Spec.Replication.Replica.Gtid
	}
	gtidString, err := gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}

	changeMasterOpts := &mariadbclient.ChangeMasterOpts{
		Connection: connectionName,
		Host: statefulset.PodFQDNWithService(
			mariadb.ObjectMeta,
			primaryPodIndex,
			ctrlresources.InternalServiceKey(mariadb).Name,
		),
		User:     replUser,
		Password: string(replSecret.Data[replPasswordRef.secretKey]),
		Gtid:     gtidString,
		Retries:  mariadb.Spec.Replication.Replica.ConnectionRetries,
	}
	if err := client.ChangeMaster(ctx, changeMasterOpts); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) reconcilePrimarySql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *mariadbclient.Client) error {
	if mariadb.Spec.Username != nil && mariadb.Spec.PasswordSecretKeyRef != nil {
		password, err := r.refResolver.SecretKeyRef(ctx, *mariadb.Spec.PasswordSecretKeyRef, mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting password: %v", err)
		}
		userOpts := mariadbclient.CreateUserOpts{
			IdentifiedBy: password,
		}
		accountName := fmt.Sprintf("'%s'@'%s'", *mariadb.Spec.Username, "%")
		if err := client.CreateUser(ctx, accountName, userOpts); err != nil {
			return fmt.Errorf("error creating user: %v", err)
		}

		grantOpts := mariadbclient.GrantOpts{
			Privileges:  []string{"ALL PRIVILEGES"},
			Database:    "*",
			Table:       "*",
			Username:    *mariadb.Spec.Username,
			GrantOption: false,
		}
		if err := client.Grant(ctx, grantOpts); err != nil {
			return fmt.Errorf("error creating grant: %v", err)
		}
	}

	if mariadb.Spec.Database != nil {
		databaseOpts := mariadbclient.DatabaseOpts{
			CharacterSet: "utf8",
			Collate:      "utf8_general_ci",
		}
		if err := client.CreateDatabase(ctx, *mariadb.Spec.Database, databaseOpts); err != nil {
			return fmt.Errorf("error creating database: %v", err)
		}
	}

	opts := userSqlOpts{
		username:   replUser,
		privileges: []string{"REPLICATION REPLICA"},
	}
	if err := r.reconcileUserSql(ctx, mariadb, client, &opts); err != nil {
		return fmt.Errorf("error reconciling '%s' SQL user: %v", replUser, err)
	}
	return nil
}

type userSqlOpts struct {
	username   string
	privileges []string
}

func (r *ReplicationConfig) reconcileUserSql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *mariadbclient.Client,
	opts *userSqlOpts) error {
	replPasswordRef := newReplPasswordRef(mariadb)
	var replPassword string
	if mariadb.Spec.Replication.Replica.ReplPasswordSecretKeyRef != nil {
		password, err := r.refResolver.SecretKeyRef(ctx, *replPasswordRef.SecretKeyRef(), mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting replication password: %v", err)
		}
		replPassword = password
	} else {
		password, err := r.secretReconciler.ReconcileRandomPassword(ctx, replPasswordRef.NamespacedName, replPasswordRef.secretKey, mariadb)
		if err != nil {
			return fmt.Errorf("error reconciling replication passsword: %v", err)
		}
		replPassword = password
	}

	exists, err := client.UserExists(ctx, replUser)
	if err != nil {
		return fmt.Errorf("error checking if replication user exists: %v", err)
	}
	if exists {
		if err := client.AlterUser(ctx, opts.username, replPassword); err != nil {
			return fmt.Errorf("error altering replication user: %v", err)
		}
	} else {
		userOpts := mariadbclient.CreateUserOpts{
			IdentifiedBy: replPassword,
		}
		accountName := fmt.Sprintf("'%s'@'%s'", opts.username, "%")
		if err := client.CreateUser(ctx, accountName, userOpts); err != nil {
			return fmt.Errorf("error creating replication user: %v", err)
		}
	}

	grantOpts := mariadbclient.GrantOpts{
		Privileges:  opts.privileges,
		Database:    "*",
		Table:       "*",
		Username:    opts.username,
		GrantOption: false,
	}
	if err := client.Grant(ctx, grantOpts); err != nil {
		return fmt.Errorf("error creating grant: %v", err)
	}
	return nil
}

type replPasswordRef struct {
	types.NamespacedName
	secretKey string
}

func newReplPasswordRef(mariadb *mariadbv1alpha1.MariaDB) replPasswordRef {
	key := types.NamespacedName{
		Name:      fmt.Sprintf("repl-password-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
	secretKey := "password"

	if mariadb.Spec.Replication != nil && mariadb.Spec.Replication.Replica.ReplPasswordSecretKeyRef != nil {
		key = types.NamespacedName{
			Name:      mariadb.Spec.Replication.Replica.ReplPasswordSecretKeyRef.Name,
			Namespace: mariadb.Namespace,
		}
		secretKey = mariadb.Spec.Replication.Replica.ReplPasswordSecretKeyRef.Key
	}

	return replPasswordRef{
		NamespacedName: key,
		secretKey:      secretKey,
	}
}

func (r replPasswordRef) SecretKeyRef() *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: r.Name,
		},
		Key: r.secretKey,
	}
}

func serverId(index int) string {
	return fmt.Sprint(10 + index)
}

func binaryFromBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
