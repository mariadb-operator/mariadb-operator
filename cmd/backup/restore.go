package backup

import (
	"fmt"
	"os"

	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
	"github.com/spf13/cobra"
)

var restoreCommand = &cobra.Command{
	Use:   "restore",
	Short: "Restore.",
	Long:  `Finds the backup file to be restored in order to implement point in time recovery.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := setupLogger(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Starting restore")

		targetTime, err := getTargetTime()
		if err != nil {
			logger.Error(err, "error getting target time")
			os.Exit(1)
		}
		logger.Info("Target time", "time", targetTime.String())

		backupFileNames, err := getBackupFileNames()
		if err != nil {
			logger.Error(err, "error reading backup files", "path", path)
			os.Exit(1)
		}

		backupTargetFile, err := backup.GetBackupTargetFile(backupFileNames, targetTime, logger.WithName("point-in-time-recovery"))
		if err != nil {
			logger.Error(err, "error reading getting target recovery file")
			os.Exit(1)
		}
		backupTargetFilepath := fmt.Sprintf("%s/%s", path, backupTargetFile)
		logger.Info("Target file", "time", backupTargetFilepath)

		if err := os.WriteFile(targetFilePath, []byte(backupTargetFilepath), 0644); err != nil {
			logger.Error(err, "error writing target file")
			os.Exit(1)
		}
	},
}

func getBackupFileNames() ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, e := range entries {
		name := e.Name()
		if backup.IsValidBackupFile(name) {
			fileNames = append(fileNames, name)
		} else {
			logger.V(1).Info("ignoring file", "file", name)
		}
	}
	return fileNames, nil
}
