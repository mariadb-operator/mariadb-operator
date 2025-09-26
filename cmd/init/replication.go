package init

import (
	"fmt"
	"os"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/spf13/cobra"
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

		if err := createReplicationConfig(env, fileManager); err != nil {
			logger.Error(err, "error creating replication configuration")
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
