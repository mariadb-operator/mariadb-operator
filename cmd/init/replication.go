package init

import (
	"context"
	"fmt"
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	replicationConfigFile = "0-replication.cnf"
	// masterInfoFileName is a file where the replicas keep the connection details to the master.
	masterInfoFileName = "master.info"
	// RelayLogFileName is the log file where the replicas keep a record of the transactions synced from the primary.
	// See: https://mariadb.com/docs/server/server-management/server-monitoring-logs/binary-log/relay-log.
	relayLogFileName = "relay-log.info"
)

var replicationCommand = &cobra.Command{
	Use:   "replication",
	Short: "Replication.",
	Long:  "Init replication instances.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Starting replication init")

		ctx, cancel := newContext()
		defer cancel()

		env, err := environment.GetPodEnv(ctx)
		if err != nil {
			logger.Error(err, "Error getting environment variables")
			os.Exit(1)
		}
		fileManager, err := filemanager.NewFileManager(configDir, stateDir)
		if err != nil {
			logger.Error(err, "Error creating file manager")
			os.Exit(1)
		}
		k8sClient, err := getK8sClient()
		if err != nil {
			logger.Error(err, "Error getting Kubernetes client")
			os.Exit(1)
		}

		if err := createReplicationConfig(env, fileManager); err != nil {
			logger.Error(err, "error creating replication configuration")
			os.Exit(1)
		}

		key := types.NamespacedName{
			Name:      env.MariadbName,
			Namespace: env.PodNamespace,
		}
		var mdb mariadbv1alpha1.MariaDB
		if err := k8sClient.Get(ctx, key, &mdb); err != nil {
			logger.Error(err, "Error getting MariaDB")
			os.Exit(0)
		}

		if err := waitForReplicaRecovery(ctx, env, &mdb, k8sClient); err != nil {
			logger.Error(err, "error waiting for replica recovery")
			os.Exit(1)
		}
		if err := cleanupReplicaState(fileManager, &mdb); err != nil {
			logger.Error(err, "error cleaning up replica state")
			os.Exit(1)
		}
		logger.Info("Replication init done")
	},
}

func createReplicationConfig(env *environment.PodEnvironment, fileManager *filemanager.FileManager) error {
	replConfig, err := replication.NewReplicationConfig(env)
	if err != nil {
		return err
	}
	logger.Info("Configuring replication")
	return fileManager.WriteConfigFile(replicationConfigFile, replConfig)
}

func waitForReplicaRecovery(ctx context.Context, env *environment.PodEnvironment, mdb *mariadbv1alpha1.MariaDB,
	client ctrlclient.Client) error {
	if !mdb.IsReplicaBeingRecovered(env.PodName) {
		return nil
	}
	logger.Info("Waiting for replica recovery")
	key := ctrlclient.ObjectKeyFromObject(mdb)

	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(context.Context) (bool, error) {
		var mariadb mariadbv1alpha1.MariaDB
		if err := client.Get(ctx, key, &mariadb); err != nil {
			return false, fmt.Errorf("error getting MariaDB: %v", err)
		}
		if mariadb.IsReplicaBeingRecovered(env.PodName) {
			logger.V(1).Info("Replica is being recovered")
			return false, nil
		}
		return true, nil
	})
}

// Cleanup previous replica state files during initialization
func cleanupReplicaState(fm *filemanager.FileManager, mdb *mariadbv1alpha1.MariaDB) error {
	if mdb.HasConfiguredReplication() || mdb.IsSwitchingPrimary() {
		return nil
	}
	logger.Info("Cleaning up replica state")

	for _, file := range []string{masterInfoFileName, relayLogFileName} {
		if err := cleanupStateFile(fm, file); err != nil {
			return err
		}
	}
	return nil
}
