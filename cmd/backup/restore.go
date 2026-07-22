package backup

import (
	"fmt"
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	"github.com/spf13/cobra"
)

var (
	targetTimeRaw             string
	targetTimeAgeThresholdRaw string
)

func init() {
	restoreCommand.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the backup target time.")
	restoreCommand.Flags().StringVar(&targetTimeAgeThresholdRaw, "target-time-age-threshold", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the target time age threshold.")
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

		if err := cleanupStaleStagingArea(); err != nil {
			logger.Error(err, "error cleaning up stale staging area")
			os.Exit(1)
		}

		backupProcessor, err := getBackupProcessor()
		if err != nil {
			logger.Error(err, "error getting backup processor")
			os.Exit(1)
		}

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

		targetTimeAgeThreshold, err := getTargetTimeAgeThreshold()
		if err != nil {
			logger.Error(err, "error getting target time age threshold")
			os.Exit(1)
		}
		if targetTimeAgeThreshold != nil {
			logger.Info("obtained target time age threshold", "threshold", targetTimeAgeThreshold.String())
		} else {
			logger.Info("no target time age threshold provided")
		}

		backupFileNames, err := backupStorage.List(ctx)
		if err != nil {
			logger.Error(err, "error listing backup files")
			os.Exit(1)
		}
		backupTargetFile, err := backupProcessor.GetBackupTargetFile(backupFileNames, targetTime,
			targetTimeAgeThreshold, logger.WithName("target-recovery-time"))
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

// cleanupStaleStagingArea cleans up /backup/full staging directory in case that it has been used before.
// Manually provisioned backup PVCs use /backup/full directory and do not clean it up after the restoration,
// potentially leading to the usage of a stale backup in further restorations.
// See: https://github.com/mariadb-operator/mariadb-operator/pull/1744
func cleanupStaleStagingArea() error {
	if backupContentType != string(mariadbv1alpha1.BackupContentTypePhysical) || physicalBackupDirPath == "" {
		return nil
	}
	logger.Info("checking stale staging area", "dir-path", physicalBackupDirPath)

	entries, err := os.ReadDir(physicalBackupDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error reading staging area directory path (%s): %v", physicalBackupDirPath, err)
	}
	if len(entries) > 0 {
		logger.Info("cleaning up staging area", "dir-path", physicalBackupDirPath)

		if err := os.RemoveAll(physicalBackupDirPath); err != nil {
			return fmt.Errorf("error removing staging area directory path (%s): %v", physicalBackupDirPath, err)
		}
	}
	return nil
}

func getTargetTime() (time.Time, error) {
	if targetTimeRaw == "" {
		return time.Now(), nil
	}
	return backup.ParseBackupDate(targetTimeRaw)
}

func getTargetTimeAgeThreshold() (*time.Time, error) {
	if targetTimeAgeThresholdRaw == "" {
		return nil, nil
	}
	t, err := backup.ParseBackupDate(targetTimeAgeThresholdRaw)
	if err != nil {
		return nil, fmt.Errorf("error parsing target time age threshold: %v", err)
	}
	return &t, nil
}

func writeTargetFile(backupTargetFile string) error {
	return os.WriteFile(targetFilePath, []byte(backupTargetFile), 0777)
}

func getBackupCompressorWithFile(fileName string, processor backup.BackupProcessor) (mdbcompression.BackupCompressor, error) {
	calg, err := processor.ParseCompressionAlgorithm(fileName)
	if err != nil {
		return nil, fmt.Errorf("error parsing compression algorithm: %v", err)
	}
	return mdbcompression.NewBackupCompressor(calg, path, processor.GetUncompressedBackupFile, logger)
}
