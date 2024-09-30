package backup

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dsnet/compress/bzip2"
	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
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
	Long:  `Finds the target backup file to implement point in time recovery.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting restore")

		ctx, cancel := newContext()
		defer cancel()

		backupStorage, err := getBackupStorage()
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

		backupTargetFile, err := backup.GetBackupTargetFile(backupFileNames, targetTime, logger.WithName("target-recovery-time"))
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

		backupFile, err := uncompressFile(path, backupTargetFile, logger.WithName("uncompress"))
		if err != nil {
			logger.Error(err, "error uncompressing file", "path", backupTargetFile)
			os.Exit(1)
		}

		logger.Info("writing target file", "path", targetFilePath)
		if err := writeTargetFile(backupFile); err != nil {
			logger.Error(err, "error writing target file", "path", targetFilePath)
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

func writeTargetFile(backupTargetFilePath string) error {
	return os.WriteFile(targetFilePath, []byte(backupTargetFilePath), 0777)
}

func uncompressFile(path string, fileName string, logger logr.Logger) (string, error) {
	calg, err := backup.ParseCompressionAlgorithm(fileName)
	if err != nil {
		return "", fmt.Errorf("error parsing compression algorithm: %v", err)
	}
	if calg == mariadbv1alpha1.CompressNone {
		return fileName, nil
	}

	fullPathFileName := filepath.Join(path, fileName)
	logger.Info("uncompressing file", "path", fullPathFileName)

	compressedFile, err := os.Open(fullPathFileName)
	if err != nil {
		return "", err
	}
	defer compressedFile.Close()

	plainFileName, err := backup.GetUncompressedBackupFile(fileName)
	if err != nil {
		return "", fmt.Errorf("error getting uncompressed file: %v", err)
	}
	plainFile, err := os.Create(filepath.Join(path, plainFileName))
	if err != nil {
		return "", err
	}
	defer plainFile.Close()

	switch calg {

	case mariadbv1alpha1.CompressGzip:
		reader, err := gzip.NewReader(compressedFile)
		if err != nil {
			return "", err
		}
		defer reader.Close()
		if _, err = io.Copy(plainFile, reader); err != nil {
			return "", err
		}

	case mariadbv1alpha1.CompressBzip2:
		reader, err := bzip2.NewReader(compressedFile,
			&bzip2.ReaderConfig{})
		if err != nil {
			return "", err
		}
		defer reader.Close()
		if _, err = io.Copy(plainFile, reader); err != nil {
			return "", err
		}
	}

	return plainFileName, nil
}
