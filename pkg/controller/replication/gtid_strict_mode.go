package replication

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PauseGtidStrictMode(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, sqlClient *sql.Client,
	k8sClient client.Client, logger logr.Logger) error {
	if !mdb.IsReplicationEnabled() {
		return nil
	}
	repl := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	if repl.GtidStrictModePaused != nil && *repl.GtidStrictModePaused {
		return nil
	}

	gtidStrictMode, err := sqlClient.GtidStrictMode(ctx)
	if err != nil {
		return fmt.Errorf("error getting gtid_strict_mode: %v", err)
	}
	if !gtidStrictMode {
		return nil
	}

	logger.Info("Temporarily disabling gtid_strict_mode")
	if err := sqlClient.DisableGtidStrictMode(ctx); err != nil {
		return fmt.Errorf("error disabling gtid_strict_mode: %v", err)
	}
	return patchStatus(ctx, mdb, k8sClient, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if status.Replication == nil {
			status.Replication = &mariadbv1alpha1.ReplicationStatus{}
		}
		status.Replication.GtidStrictModePaused = ptr.To(true)
		return nil
	})
}

func ResumeGtidStrictMode(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, sqlClient *sql.Client,
	k8sClient client.Client, logger logr.Logger) error {
	if !mdb.IsReplicationEnabled() {
		return nil
	}
	repl := ptr.Deref(mdb.Status.Replication, mariadbv1alpha1.ReplicationStatus{})
	if repl.GtidStrictModePaused == nil || !*repl.GtidStrictModePaused {
		return nil
	}

	logger.Info("Enabling back gtid_strict_mode")
	if err := sqlClient.EnableGtidStrictMode(ctx); err != nil {
		return fmt.Errorf("error enabling gtid_strict_mode: %v", err)
	}
	return patchStatus(ctx, mdb, k8sClient, func(status *mariadbv1alpha1.MariaDBStatus) error {
		if status.Replication != nil {
			status.Replication.GtidStrictModePaused = nil
		}
		return nil
	})
}

func patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	k8sClient client.Client, patcher func(*mariadbv1alpha1.MariaDBStatus) error) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	if err := patcher(&mariadb.Status); err != nil {
		return err
	}
	return k8sClient.Status().Patch(ctx, mariadb, patch)
}
