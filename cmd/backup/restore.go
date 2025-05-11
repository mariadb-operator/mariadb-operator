package backup

import (
	"fmt"
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
)

var targetTimeRaw string

func init() {
	restoreCommand.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the backup target time.")
	restoreCommand.Flags().StringVar(&path, "path", "/backup", "Directory path where the backup files are located.")
}

var restoreCommand = &cobra.Command{
	Use:   "restore",
	Short: "Restore.",
	Long:  `Fetches backup files from multiple storage types and matches the one closest to the target recovery time.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting restore")

		ctx, cancel := newContext()
		defer cancel()

		backupProcessor := getBackupProcessor()

		backupStorage, err := getBackupStorage(backupProcessor)
		if err != nil {
			logger.Error(err, "error getting backup storage")
			os.Exit(1)
		}

		targetTime, err := getTargetTime()
		if err != nil {
			logger.Error(err, "error getting target time")
			os.Exit(1)
		}
		logger.Info("obtained target time", "time", targetTime.String())

		backupFileNames, err := backupStorage.List(ctx)
		if err != nil {
			logger.Error(err, "error listing backup files")
			os.Exit(1)
		}

		backupTargetFile, err := backupProcessor.GetBackupTargetFile(backupFileNames, targetTime, logger.WithName("target-recovery-time"))
		if err != nil {
			logger.Error(err, "error reading getting target backup")
			os.Exit(1)
		}
		logger.Info("obtained target backup", "file", backupTargetFile)

		logger.Info("pulling target backup", "file", backupTargetFile, "prefix", s3Prefix)
		if err := backupStorage.Pull(ctx, backupTargetFile); err != nil {
			logger.Error(err, "error pulling target backup", "file", backupTargetFile, "prefix", s3Prefix)
			os.Exit(1)
		}

		backupCompressor, err := getBackupCompressorWithFile(backupTargetFile, backupProcessor)
		if err != nil {
			logger.Error(err, "error getting backup compressor")
			os.Exit(1)
		}
		backupTargetFile, err = backupCompressor.Decompress(backupTargetFile)
		if err != nil {
			logger.Error(err, "error decompressing backup", "file", backupTargetFile)
			os.Exit(1)
		}

		logger.Info("writing target file", "file", targetFilePath, "file-content", backupTargetFile)
		if err := writeTargetFile(backupTargetFile); err != nil {
			logger.Error(err, "error writing target file", "file", backupTargetFile)
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

func writeTargetFile(backupTargetFile string) error {
	return os.WriteFile(targetFilePath, []byte(backupTargetFile), 0777)
}

func getBackupCompressorWithFile(fileName string, processor backup.BackupProcessor) (backup.BackupCompressor, error) {
	calg, err := processor.ParseCompressionAlgorithm(fileName)
	if err != nil {
		return nil, fmt.Errorf("error parsing compression algorithm: %v", err)
	}
	return getBackupCompressorWithAlgorithm(calg, processor)
}
