package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	"github.com/spf13/cobra"
)

const operatorBinaryPath = "/bin/mariadb-operator"

var (
	targetTimeRaw string
	copyBinaryTo  string
)

func init() {
	restoreCommand.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the backup target time.")
	restoreCommand.Flags().StringVar(&copyBinaryTo, "copy-binary-to", "",
		"Path to copy the operator binary for streaming restore mode. When set, the restore command writes the S3 key "+
			"to the target file and copies the operator binary, instead of downloading and decompressing the backup.")
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

		physicalBackupExists, err := checkPhysicalBackupDir()
		if err != nil {
			logger.Error(err, "error checking physical backup directory")
			os.Exit(1)
		}
		if physicalBackupExists {
			logger.Info("physical backup directory already exists.")
			os.Exit(0)
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

		if copyBinaryTo != "" {
			// Streaming mode: write S3 key to target file and copy operator binary.
			// The main container will use the binary to invoke `backup stream | mbstream`.
			logger.Info("streaming mode: writing target file and copying operator binary",
				"target-file", targetFilePath, "copy-binary-to", copyBinaryTo)

			if err := writeTargetFile(backupTargetFile); err != nil {
				logger.Error(err, "error writing target file", "file", backupTargetFile)
				os.Exit(1)
			}
			if err := copyOperatorBinary(copyBinaryTo); err != nil {
				logger.Error(err, "error copying operator binary", "dest", copyBinaryTo)
				os.Exit(1)
			}
			logger.Info("streaming mode setup complete")
		} else {
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
		}
	},
}

func checkPhysicalBackupDir() (bool, error) {
	if backupContentType != string(mariadbv1alpha1.BackupContentTypePhysical) || physicalBackupDirPath == "" {
		return false, nil
	}
	logger.Info("checking existing physical backup directory", "dir-path", physicalBackupDirPath)

	entries, err := os.ReadDir(physicalBackupDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error reading physical backup directory path (%s): %v", physicalBackupDirPath, err)
	}
	return len(entries) > 0, nil
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

func getBackupCompressorWithFile(fileName string, processor backup.BackupProcessor) (mdbcompression.BackupCompressor, error) {
	calg, err := processor.ParseCompressionAlgorithm(fileName)
	if err != nil {
		return nil, fmt.Errorf("error parsing compression algorithm: %v", err)
	}
	return mdbcompression.NewBackupCompressor(calg, path, processor.GetUncompressedBackupFile, logger)
}

func copyOperatorBinary(destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("error creating destination directory: %v", err)
	}
	src, err := os.Open(operatorBinaryPath)
	if err != nil {
		return fmt.Errorf("error opening operator binary: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("error creating destination file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copying operator binary: %v", err)
	}
	return nil
}
