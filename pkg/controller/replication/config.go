package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationConfig struct {
	mariadb       *mariadbv1alpha1.MariaDB
	mariadbClient *mariadb.Client
	client        client.Client
}

func NewReplicationConfig(mariadb *mariadbv1alpha1.MariaDB, mariadbClient *mariadb.Client,
	client client.Client) *ReplicationConfig {
	return &ReplicationConfig{
		mariadb:       mariadb,
		mariadbClient: mariadbClient,
		client:        client,
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
	if err := r.configurepPrimary(ctx, r.mariadb, r.mariadbClient, podIndex); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
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
	if err := r.mariadbClient.SetGlobalVar(ctx, "read_only", "1"); err != nil {
		return fmt.Errorf("error setting read_only=1: %v", err)
	}
	if err := r.configurepReplica(ctx, r.mariadb, r.mariadbClient, replicaPodIndex); err != nil {
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

func (r *ReplicationConfig) configurepPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	client *mariadb.Client, ordinal int) error {
	kv := map[string]string{
		"rpl_semi_sync_master_enabled": "ON",
		"rpl_semi_sync_master_timeout": func() string {
			return fmt.Sprint(mariadb.Spec.Replication.Replica.ConnectionTimeoutOrDefault().Milliseconds())
		}(),
		"rpl_semi_sync_slave_enabled": "OFF",
		"server_id":                   serverId(ordinal),
	}
	if mariadb.Spec.Replication.Replica.WaitPoint != nil {
		waitPoint, err := mariadb.Spec.Replication.Replica.WaitPoint.MariaDBFormat()
		if err != nil {
			return fmt.Errorf("error getting wait point: %v", err)
		}
		kv["rpl_semi_sync_master_wait_point"] = waitPoint
	}
	if err := client.SetGlobalVars(ctx, kv); err != nil {
		return fmt.Errorf("error setting replication vars: %v", err)
	}
	return nil
}

func (r *ReplicationConfig) configurepReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
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
	if err := r.client.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
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

func serverId(index int) string {
	return fmt.Sprint(10 + index)
}
