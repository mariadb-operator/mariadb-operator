package init

import (
	"fmt"
	"os"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	replicationresources "github.com/mariadb-operator/mariadb-operator/v25/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
)

const replicationConfigFile = "0-replication.cnf"

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
		podIndex, err := statefulset.PodIndex(env.PodName)
		if err != nil {
			logger.Error(err, "error getting index from Pod", "pod", env.PodName)
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

		if err := cleanupReplicaState(fileManager, &mdb, *podIndex); err != nil {
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

// Cleanup replica state files to prevent starting a potential primary as replica due to previous state
func cleanupReplicaState(fm *filemanager.FileManager, mdb *mariadbv1alpha1.MariaDB, podIndex int) error {
	if mdb.Status.CurrentPrimaryPodIndex == nil ||
		(mdb.Status.CurrentPrimaryPodIndex != nil && *mdb.Status.CurrentPrimaryPodIndex != podIndex) ||
		mdb.IsSwitchingPrimary() {
		return nil
	}
	logger.Info("Cleaning up replica state")

	for _, file := range []string{replicationresources.MasterInfoFileName, replicationresources.RelayLogFileName} {
		if err := cleanupStateFile(fm, file); err != nil {
			return err
		}
	}
	return nil
}
