package backup

import (
	"bufio"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

		logger.Info("uncompressing file", "path", backupTargetFile)
		if err := uncompressFile(filepath.Join(path, backupTargetFile) ); err != nil {
			logger.Error(err, "error uncompressing file", "path", backupTargetFile)
			os.Exit(1)
		}

		logger.Info("writing target file", "path", targetFilePath)
		if err := writeTargetFile(backupTargetFile); err != nil {
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

func uncompressFile(f string) error {
	tmpf := f + ".tmp"
	err := os.Rename(f,tmpf)
	if err != nil {
		panic(err)
	}
	compressedFile, err := os.Open(tmpf)
	if err != nil {
		return (err)
	}
	defer compressedFile.Close()

	originalFile, err := os.Create(f)
	if err != nil {
		return (err)
	}
	defer originalFile.Close()

	bReader := bufio.NewReader(compressedFile)
	testBytes, err := bReader.Peek(2)
	if err != nil {
		return (err)
	}
	if testBytes[0] == 31 && testBytes[1] == 139 {
		//gzip
		logger.Info("found gzip compression")
		reader, err := gzip.NewReader(bReader)
		if err != nil {
			return (err)
		}
		defer reader.Close()
		_, err = io.Copy(originalFile, reader)
		if err != nil {
			return (err)
		}
	} else if testBytes[0] == 120 && (testBytes[1] == 1 || testBytes[1] == 156 || testBytes[1] == 218) {
		// zlib
		logger.Info("found zlib compression")
		reader, err := zlib.NewReader(bReader)
		if err != nil {
			return (err)
		}
		defer reader.Close()
		_, err = io.Copy(originalFile, reader)
		if err != nil {
			return (err)
		}
	} else {
		// no compression, do nothing
		_, err = io.Copy(originalFile, bReader)
		if err != nil {
			return (err)
		}
	}
	err = os.Remove(tmpf)
	if err != nil {
		return (err)
	}
	return nil
}
