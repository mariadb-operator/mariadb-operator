package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger               = ctrl.Log
	path                 string
	targetFilePath       string
	maxRetentionDuration time.Duration
)

func init() {
	RootCmd.PersistentFlags().StringVar(&path, "path", "/backup", "Directory path where the backup files are located.")
	RootCmd.PersistentFlags().StringVar(&targetFilePath, "target-file-path", "/backup/0-backup-target.txt",
		"Path to a file that contains the name of the backup target file.")

	RootCmd.Flags().DurationVar(&maxRetentionDuration, "max-retention-duration", 30*24*time.Hour,
		"Defines the retention policy for backups. Older backups will be deleted.")

	RootCmd.AddCommand(restoreCommand)
}

var RootCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup.",
	Long:  `Manages the backed up files in order to be compliant with the retention policy.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := setupLogger(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting backup",
			"path", path, "target-file-path", targetFilePath, "max-retention-duration", maxRetentionDuration)

		backupNames, err := getBackupFileNames()
		if err != nil {
			logger.Error(err, "error reading backup files", "path", path)
			os.Exit(1)
		}

		backupsToDelete := backup.GetBackupFilesToDelete(backupNames, maxRetentionDuration, logger.WithName("backup-cleanup"))
		if len(backupsToDelete) == 0 {
			logger.Info("no old backups were found")
			os.Exit(0)
		}
		logger.Info("old backups to delete", "backups", len(backupsToDelete))

		for _, backup := range backupsToDelete {
			backupPath := getBackupPath(backup)
			logger.V(1).Info("deleting old backup", "backup", backupPath)

			if err := os.Remove(backupPath); err != nil {
				logger.Error(err, "error removing old backup", "backup", backupPath)
			}
		}
	},
}

func setupLogger(cmd *cobra.Command) error {
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return fmt.Errorf("error getting 'log-level' flag: %v\n", err)
	}
	logTimeEncoder, err := cmd.Flags().GetString("log-time-encoder")
	if err != nil {
		return fmt.Errorf("error getting 'log-time-encoder' flag: %v\n", err)
	}
	logDev, err := cmd.Flags().GetBool("log-dev")
	if err != nil {
		return fmt.Errorf("error getting 'log-dev' flag: %v\n", err)
	}
	log.SetupLogger(logLevel, logTimeEncoder, logDev)
	return nil
}

func getBackupFileNames() ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, e := range entries {
		name := e.Name()
		logger.V(1).Info("processing backup file", "file", name)
		if backup.IsValidBackupFile(name) {
			fileNames = append(fileNames, name)
		} else {
			logger.V(1).Info("ignoring file", "file", name)
		}
	}
	return fileNames, nil
}

func getBackupPath(backupFileName string) string {
	return filepath.Join(path, backupFileName)
}
