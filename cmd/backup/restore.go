package backup

import (
	"fmt"
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
	"github.com/spf13/cobra"
)

var targetTimeRaw string

func init() {
	restoreCommand.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the backup target time.")
}

var restoreCommand = &cobra.Command{
	Use:   "restore",
	Short: "Restore.",
	Long:  `Finds the backup file to be restored in order to implement point in time recovery.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := setupLogger(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting restore",
			"path", path, "target-file-path", targetFilePath, "target-time", targetTimeRaw)

		targetTime, err := getTargetTime()
		if err != nil {
			logger.Error(err, "error getting target time")
			os.Exit(1)
		}
		logger.Info("target time", "time", targetTime.String())

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
		backupTargetFilepath := getBackupPath(backupTargetFile)

		logger.Info("writing target file", "file", backupTargetFilepath)
		if err := os.WriteFile(targetFilePath, []byte(backupTargetFilepath), 0777); err != nil {
			logger.Error(err, "error writing target file")
			os.Exit(1)
		}
	},
}

func getTargetTime() (time.Time, error) {
	if targetTimeRaw == "" {
		return time.Now(), nil
	}
	return backup.ParseBackupDate(targetTimeRaw)
}
