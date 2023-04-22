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
	if err := config.client.SetGlobalVar(ctx, "read_only", "0"); err != nil {
		return fmt.Errorf("error setting read_only=0: %v", err)
	}
	if err := r.configureReplicationVars(ctx, config.mariadb, config.client, config.ordinal); err != nil {
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
	if err := config.client.SetGlobalVar(ctx, "read_only", "1"); err != nil {
		return fmt.Errorf("error setting read_only=1: %v", err)
	}
	if err := r.configureReplicationVars(ctx, config.mariadb, config.client, config.ordinal); err != nil {
		return fmt.Errorf("error configuring replication variables: %v", err)
	}
	if err := config.client.ResetMaster(ctx); err != nil {
		return fmt.Errorf("error resetting master: %v", err)
	}
	if err := config.client.StopAllSlaves(ctx); err != nil {
		return fmt.Errorf("error stopping slaves: %v", err)
	}
	if err := config.client.ChangeMaster(ctx, config.changeMasterOpts); err != nil {
		return fmt.Errorf("error changing master: %v", err)
	}
	if err := config.client.StartSlave(ctx, ConnectionName); err != nil {
		return fmt.Errorf("error starting slave: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) configureReplicationVars(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	client *mariadb.Client, ordinal int) error {
	kv := map[string]string{
		"rpl_semi_sync_master_enabled": "ON",
		"rpl_semi_sync_slave_enabled":  "ON",
		"rpl_semi_sync_master_timeout": func() string {
			if mariadb.Spec.Replication.Timeout != nil {
				return fmt.Sprint(mariadb.Spec.Replication.Timeout.Milliseconds())
			}
			return "30000"
		}(),
		"server_id": func() string {
			return fmt.Sprint(10 + ordinal)
		}(),
	}
	if mariadb.Spec.Replication.WaitPoint != nil {
		waitPoint, err := mariadb.Spec.Replication.WaitPoint.MariaDBFormat()
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
