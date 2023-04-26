package replication

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
)

type primaryConfig struct {
	mariadb *mariadbv1alpha1.MariaDB
	client  *mariadb.Client
	ordinal int
}

func (r *ReplicationReconciler) configurePrimary(ctx context.Context, config *primaryConfig) error {
	if err := config.client.UnlockTables(ctx); err != nil {
		return fmt.Errorf("error unlocking tables: %v", err)
	}
	if err := config.client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := config.client.ResetAllSlaves(ctx); err != nil {
		return fmt.Errorf("error resetting slave: %v", err)
	}
	if err := config.client.ResetSlavePos(ctx); err != nil {
		return fmt.Errorf("error resetting slave_pos: %v", err)
	}
	if err := config.client.SetGlobalVar(ctx, "read_only", "0"); err != nil {
		return fmt.Errorf("error setting read_only=0: %v", err)
	}
	if err := r.configurepPrimaryReplication(ctx, config.mariadb, config.client, config.ordinal); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	return nil
}

type replicaConfig struct {
	mariadb          *mariadbv1alpha1.MariaDB
	client           *mariadb.Client
	changeMasterOpts *mariadbclient.ChangeMasterOpts
	ordinal          int
}

func (r *ReplicationReconciler) configureReplica(ctx context.Context, config *replicaConfig) error {
	if err := config.client.UnlockTables(ctx); err != nil {
		return fmt.Errorf("error unlocking tables: %v", err)
	}
	if err := config.client.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := config.client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := config.client.SetGlobalVar(ctx, "read_only", "1"); err != nil {
		return fmt.Errorf("error setting read_only=1: %v", err)
	}
	if err := r.configurepReplicaReplication(ctx, config.mariadb, config.client, config.ordinal); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := config.client.ChangeMaster(ctx, config.changeMasterOpts); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := config.client.StartSlave(ctx, ConnectionName); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) configurepPrimaryReplication(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
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

func (r *ReplicationReconciler) configurepReplicaReplication(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
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

func serverId(ordinal int) string {
	return fmt.Sprint(10 + ordinal)
}
