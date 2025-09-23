package replication

import (
	"context"
	"fmt"
	"strconv"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	env "github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/version"
	"golang.org/x/mod/semver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	replUser     = "repl"
	replUserHost = "%"
)

type ReplicationConfig struct {
	client.Client
	builder          *builder.Builder
	refResolver      *refresolver.RefResolver
	secretReconciler *secret.SecretReconciler
	env              *env.OperatorEnv
}

func NewReplicationConfig(client client.Client, builder *builder.Builder, secretReconciler *secret.SecretReconciler,
	env *env.OperatorEnv) *ReplicationConfig {
	return &ReplicationConfig{
		Client:           client,
		builder:          builder,
		refResolver:      refresolver.New(client),
		secretReconciler: secretReconciler,
		env:              env,
	}
}

func (r *ReplicationConfig) ConfigurePrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
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

func (r *ReplicationConfig) ConfigureReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	replicaPodIndex, primaryPodIndex int, resetSlavePos bool) error {
	replication := mariadb.Replication()
	isExternalReplication := replication.IsExternalReplication()

	if err := client.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if resetSlavePos {
		if err := client.ResetSlavePos(ctx); err != nil {
			return fmt.Errorf("error resetting slave position: %v", err)
		}
	}
	if err := client.EnableReadOnly(ctx); err != nil {
		return fmt.Errorf("error enabling read_only: %v", err)
	}

	isReplicationConfigured, _ := client.IsReplicationConfigured(ctx)

	// return fmt.Errorf("REPLICATION ALREADY configured :%v ", val)

	if isExternalReplication && !isReplicationConfigured {

		// Get external mariadb
		emdb, err := r.refResolver.ExternalMariaDB(ctx, &replication.ReplicaFromExternal.MariaDBRef, mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting external MariaDB object: %v", err)
		}
		key := types.NamespacedName{
			Name:      emdb.Name,
			Namespace: emdb.Namespace,
		}
		// Check if a viable backup already exists
		var isBackupInvalid = false
		var binlogExpireLogsDuration time.Duration
		var existingBackup mariadbv1alpha1.Backup

		if binlogExpireLogsDuration, err = getBinlogExpireLogsDuration(emdb, ctx, r.refResolver); err != nil {
			return fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
		}
		err = r.Get(ctx, key, &existingBackup)
		if err == nil {
			isBackupInvalid = invalidateBackup(existingBackup, ctx, binlogExpireLogsDuration, *r)
		}
		// Create a new backup if required
		if err != nil || isBackupInvalid {
			return newBackup(emdb, *r, ctx, binlogExpireLogsDuration, mariadb.GetImagePullSecrets(), mariadb.Spec.Storage.Size)
		}

		if !existingBackup.IsComplete() {
			return nil
		}

		var existingRestore mariadbv1alpha1.Restore
		err = r.Get(ctx, mariadb.RestoreKeyInPod(replicaPodIndex), &existingRestore)

		if err == nil && !existingRestore.IsComplete() {
			return nil
		}

		if !existingRestore.IsComplete() {
			// Restore/Bootstrap node from backup
			return newRestore(mariadb, *r, ctx, replicaPodIndex)
		}

		if err := r.Delete(ctx, &existingRestore); err != nil {
			return fmt.Errorf("error deleting Restore: %v", err)
		}

	}

	if err := r.configureReplicaVars(ctx, mariadb, client, replicaPodIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := r.changeMaster(ctx, mariadb, client, primaryPodIndex); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := client.StartSlave(ctx); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configurePrimaryVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	primaryPodIndex int) error {

	kv := map[string]string{
		"sync_binlog":                  fmt.Sprintf("%d", ptr.Deref(mariadb.Replication().SyncBinlog, 1)),
		"rpl_semi_sync_master_enabled": "ON",
		"rpl_semi_sync_master_timeout": func() string {
			return fmt.Sprint(mariadb.Replication().Replica.ConnectionTimeout.Milliseconds())
		}(),
		"rpl_semi_sync_slave_enabled": "OFF",
		"server_id":                   serverId(primaryPodIndex),
	}
	if mariadb.Replication().Replica.WaitPoint != nil {
		waitPoint, err := mariadb.Replication().Replica.WaitPoint.MariaDBFormat()
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
	client *sql.Client, ordinal int) error {
	var server_id string

	if mariadb.Replication().ReplicaFromExternal != nil {
		server_id = offsetServerId(ordinal, *mariadb.Replication().ReplicaFromExternal.ServerIdOffset)

	} else {
		server_id = serverId(ordinal)
	}

	kv := map[string]string{
		"sync_binlog":                  fmt.Sprintf("%d", ptr.Deref(mariadb.Replication().SyncBinlog, 1)),
		"rpl_semi_sync_master_enabled": "OFF",
		"rpl_semi_sync_slave_enabled":  "ON",
		"server_id":                    server_id,
	}
	if err := client.SetSystemVariables(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) changeMaster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
	primaryPodIndex int) error {
	replication := mariadb.Replication()

	gtid := mariadbv1alpha1.GtidCurrentPos
	if mariadb.Replication().Replica.Gtid != nil {
		gtid = *mariadb.Replication().Replica.Gtid
	}
	gtidString, err := gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}

	changeMasterHostOpt, err := r.getChangeMasterHost(ctx, mariadb, primaryPodIndex)
	if err != nil {
		return fmt.Errorf("error getting host option: %v", err)
	}

	var changeMasterOpts []sql.ChangeMasterOpt

	if !replication.IsExternalReplication() {
		replPasswordRef := newReplPasswordRef(mariadb)
		password, err := r.refResolver.SecretKeyRef(ctx, replPasswordRef.SecretKeySelector, mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting replication password: %v", err)
		}
		changeMasterOpts = []sql.ChangeMasterOpt{
			changeMasterHostOpt,
			sql.WithChangeMasterPort(mariadb.Spec.Port),
			sql.WithChangeMasterCredentials(replUser, password),
			sql.WithChangeMasterGtid(gtidString),
			sql.WithChangeMasterRetries(*mariadb.Replication().Replica.ConnectionRetries),
		}

		if mariadb.IsTLSEnabled() {
			changeMasterOpts = append(changeMasterOpts, sql.WithChangeMasterSSL(
				builderpki.ClientCertPath,
				builderpki.ClientKeyPath,
				builderpki.CACertPath,
			))
		}
	} else {
		var emdb *mariadbv1alpha1.ExternalMariaDB
		replPasswordRef, err := externalReplPasswordRef(mariadb, r.refResolver, ctx)
		if err != nil {
			return fmt.Errorf("error getting ExternalMariaDB password Ref: %v", err)
		}
		password, err := r.refResolver.SecretKeyRef(ctx, replPasswordRef, mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting ExternalMariaDB password replication secret: %v", err)
		}
		emdbRef := replication.GetExternalReplicationRef()
		emdb, err = r.refResolver.ExternalMariaDB(ctx, &emdbRef, mariadb.Namespace)
		if err != nil {
			return fmt.Errorf("error getting ExternalMariaDB: %v", err)
		}
		changeMasterOpts = []sql.ChangeMasterOpt{
			changeMasterHostOpt,
			sql.WithChangeMasterPort(emdb.GetPort()),
			sql.WithChangeMasterCredentials(emdb.GetSUName(), password),
			// sql.WithChangeMasterGtid(gtidString),
			sql.WithChangeMasterRetries(*mariadb.Replication().Replica.ConnectionRetries),
		}
	}

	if err := client.ChangeMaster(ctx, changeMasterOpts...); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) getChangeMasterHost(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	primaryPodIndex int) (sql.ChangeMasterOpt, error) {
	replication := mariadb.Replication()
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
	if replication.IsExternalReplication() {
		emdbRef := replication.GetExternalReplicationRef()

		emdb, err := r.refResolver.ExternalMariaDB(ctx, &emdbRef, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error gettinr ExternalMariaDB: %v", err)
		}

		//TODO Check hostname length

		return sql.WithChangeMasterHost(
			emdb.GetHost(),
		), nil
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

func (r *ReplicationConfig) reconcilePrimarySql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client) error {
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

func (r *ReplicationConfig) reconcileUserSql(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, client *sql.Client,
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

func newRestore(mariadb *mariadbv1alpha1.MariaDB, r ReplicationConfig, ctx context.Context, replicaPodIndex int) error {
	restoreOpts := builder.RestoreOpts{
		PodIndex: &replicaPodIndex,
	}
	restore, err := r.builder.BuildRestore(mariadb, mariadb.RestoreKeyInPod(replicaPodIndex), restoreOpts)
	if err != nil {
		return fmt.Errorf("error building Restore object: %v", err)
	}
	if err := r.Create(ctx, restore); err != nil {
		return fmt.Errorf("error creating Restore object: %v", err)
	}
	// return fmt.Errorf("CREATING Restore object: %v", restore.Name)
	return nil
}

func newBackup(emdb *mariadbv1alpha1.ExternalMariaDB, r ReplicationConfig, ctx context.Context,
	binlogExpireLogsDuration time.Duration, imagePullSecrets []mariadbv1alpha1.LocalObjectReference,
	size *resource.Quantity) error {

	key := types.NamespacedName{
		Name:      emdb.Name,
		Namespace: emdb.Namespace,
	}
	backupOps := builder.BackupOpts{
		Metadata: []*mariadbv1alpha1.Metadata{emdb.Spec.InheritMetadata},
		Key:      key,
		MariaDBRef: mariadbv1alpha1.MariaDBRef{
			ObjectReference: mariadbv1alpha1.ObjectReference{
				Name: emdb.Name,
			},
			Kind: mariadbv1alpha1.ExternalMariaDBKind,
		},
		Args: []string{
			"--master-data=1",
			"--gtid",
			"--verbose",
			"--all-databases",
			"--single-transaction",
			"--ignore-table=mysql.global_priv",
		},
		Compression: mariadbv1alpha1.CompressGzip,
		Storage: mariadbv1alpha1.BackupStorage{
			PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteOnce,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						"storage": *size,
					},
				},
			},
		},
		MaxRetention:     binlogExpireLogsDuration,
		ImagePullSecrets: imagePullSecrets,
	}

	backup, err := r.builder.BuildBackup(backupOps, emdb)
	if err != nil {
		return fmt.Errorf("error building Backup object: %v", err)
	}
	if err := r.Create(ctx, backup); err != nil {
		return fmt.Errorf("error creating base Backup: %v", err)
	}
	return nil
}

func getBinlogExpireLogsDuration(emdb *mariadbv1alpha1.ExternalMariaDB, ctx context.Context,
	refResolver *refresolver.RefResolver) (time.Duration, error) {
	var external_client *sql.Client
	var err error
	if external_client, err = sql.NewClientWithMariaDB(ctx, emdb, refResolver); err != nil {
		return time.Duration(0), fmt.Errorf("error getting external MariaDB client: %v", err)
	}
	defer external_client.Close()

	var binlogExpireLogsSecondsStr string
	var binlogExpireLogsSeconds int

	if semver.Compare(emdb.Status.Version, "10.6.1") >= 0 {
		binlogExpireLogsSecondsStr, err = external_client.SystemVariable(ctx, "binlog_expire_logs_seconds")
		if err != nil {
			return time.Duration(0), fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
		}
		binlogExpireLogsSeconds, _ = strconv.Atoi(binlogExpireLogsSecondsStr)
	} else {
		binlogExpireLogsDaysStr, err := external_client.SystemVariable(ctx, "binlog_expire_logs_seconds")
		if err != nil {
			return time.Duration(0), fmt.Errorf("unable to get binlog_expire_logs_seconds: %v", err)
		}
		binlogExpireLogsDays, _ := strconv.Atoi(binlogExpireLogsDaysStr)
		binlogExpireLogsSeconds = binlogExpireLogsDays * 86400
	}

	return time.Duration(binlogExpireLogsSeconds) * time.Second, nil
}

func invalidateBackup(existingBackup mariadbv1alpha1.Backup, ctx context.Context,
	binlogExpireLogsDuration time.Duration, r ReplicationConfig) bool {
	if time.Since(existingBackup.CreationTimestamp.Time) > binlogExpireLogsDuration {
		if err := r.Delete(ctx, &existingBackup); err == nil {
			return true
		}
	}
	return false
}

func newReplPasswordRef(mariadb *mariadbv1alpha1.MariaDB) mariadbv1alpha1.GeneratedSecretKeyRef {
	if mariadb.Replication().Enabled && mariadb.Replication().Replica.ReplPasswordSecretKeyRef != nil {
		return *mariadb.Replication().Replica.ReplPasswordSecretKeyRef
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

func externalReplPasswordRef(mariadb *mariadbv1alpha1.MariaDB, r *refresolver.RefResolver,
	ctx context.Context) (mariadbv1alpha1.SecretKeySelector, error) {
	replication := mariadb.Replication()
	if mariadb.Replication().Enabled && mariadb.Replication().Replica.ReplPasswordSecretKeyRef != nil {
		return mariadb.Replication().Replica.ReplPasswordSecretKeyRef.SecretKeySelector, nil
	}
	if replication.IsExternalReplication() {
		emdbRef := replication.GetExternalReplicationRef()
		emdb, err := r.ExternalMariaDB(ctx, &emdbRef, mariadb.Namespace)
		if err == nil {
			return *emdb.GetSUCredential(), nil
		}
	}
	return mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: "",
		},
		Key: "",
	}, fmt.Errorf("not able to get PasswordRef for external replication")
}

func serverId(index int) string {
	return fmt.Sprint(10 + index)
}

func offsetServerId(index int, offset int) string {
	return fmt.Sprint(offset + index)
}

func formatAccountName(username, host string) string {
	return fmt.Sprintf("'%s'@'%s'", username, host)
}
