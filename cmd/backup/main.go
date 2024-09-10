package backup

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger         = ctrl.Log
	path           string
	targetFilePath string
	s3             bool
	s3Bucket       string
	s3Endpoint     string
	s3Region       string
	s3TLS          bool
	s3CACertPath   string
	s3Prefix       string
	maxRetention   time.Duration
	compression    string
)

func init() {
	RootCmd.PersistentFlags().StringVar(&path, "path", "/backup", "Directory path where the backup files are located.")
	RootCmd.PersistentFlags().StringVar(&targetFilePath, "target-file-path", "/backup/0-backup-target.txt",
		"Path to a file that contains the name of the backup target file.")
	if err := RootCmd.MarkPersistentFlagRequired("path"); err != nil {
		fmt.Printf("error marking 'path' flag as required: %v", err)
		os.Exit(1)
	}
	if err := RootCmd.MarkPersistentFlagRequired("target-file-path"); err != nil {
		fmt.Printf("error marking 'target-file-path' flag as required: %v", err)
		os.Exit(1)
	}

	RootCmd.PersistentFlags().BoolVar(&s3, "s3", false, "Enable S3 backup storage.")
	RootCmd.PersistentFlags().StringVar(&s3Bucket, "s3-bucket", "backups", "Name of the bucket to store backups.")
	RootCmd.PersistentFlags().StringVar(&s3Endpoint, "s3-endpoint", "s3.amazonaws.com", "S3 API endpoint without scheme.")
	RootCmd.PersistentFlags().StringVar(&s3Region, "s3-region", "us-east-1", "S3 region name to use.")
	RootCmd.PersistentFlags().BoolVar(&s3TLS, "s3-tls", false, "Enable S3 TLS connections.")
	RootCmd.PersistentFlags().StringVar(&s3CACertPath, "s3-ca-cert-path", "", "Path to the CA to be trusted when connecting to S3.")
	RootCmd.PersistentFlags().StringVar(&s3Prefix, "s3-prefix", "", "S3 bucket prefix name to use.")

	RootCmd.PersistentFlags().StringVar(&compression, "compression", string(mariadbv1alpha1.CompressNone),
		"Compression algorithm, none, gzip or zlib.")

	err := mariadbv1alpha1.CompressAlgorithm(compression).Validate()
	if err != nil {
		fmt.Printf("compression algorithm not supported: %v", err)
		os.Exit(1)
	}

	RootCmd.Flags().DurationVar(&maxRetention, "max-retention", 30*24*time.Hour,
		"Defines the retention policy for backups. Older backups will be deleted.")

	RootCmd.AddCommand(restoreCommand)
}

var RootCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup.",
	Long:  `Manages the backup files to implement the retention policy.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting backup")

		ctx, cancel := newContext()
		defer cancel()

		backupStorage, err := getBackupStorage()
		if err != nil {
			logger.Error(err, "error getting backup storage")
			os.Exit(1)
		}

		logger.Info("reading target file", "path", targetFilePath)
		backupTargetFile, err := readTargetFile()
		if err != nil {
			logger.Error(err, "error reading target file", "path", targetFilePath)
			os.Exit(1)
		}
		logger.Info("obtained target backup", "file", backupTargetFile)

		if compression != string(mariadbv1alpha1.CompressNone) {
			logger.Info("compressing target backup", "file", backupTargetFile)
			err := compressFile(filepath.Join(path, backupTargetFile), mariadbv1alpha1.CompressAlgorithm(compression))
			if err != nil {
				logger.Error(err, "error compressing file", "path", backupTargetFile, "error", err)
				os.Exit(1)
			}
		}

		logger.Info("pushing target backup", "file", backupTargetFile, "prefix", s3Prefix)
		if err := backupStorage.Push(ctx, backupTargetFile); err != nil {
			logger.Error(err, "error pushing target backup", "file", backupTargetFile, "prefix", s3Prefix)
			os.Exit(1)
		}

		backupNames, err := backupStorage.List(ctx)
		if err != nil {
			logger.Error(err, "error listing backup files")
			os.Exit(1)
		}

		logger.Info("cleaning up old backups")
		oldBackups := backup.GetOldBackupFiles(backupNames, maxRetention, logger.WithName("backup-cleanup"))
		if len(oldBackups) == 0 {
			logger.Info("no old backups were found")
			os.Exit(0)
		}
		logger.Info("old backups to delete", "backups", len(oldBackups))

		for _, backup := range oldBackups {
			logger.V(1).Info("deleting old backup", "backup", backup)
			if err := backupStorage.Delete(ctx, backup); err != nil {
				logger.Error(err, "error removing old backup", "backup", backup)
			}
		}
	},
}

func newContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT}...,
	)
}

func getBackupStorage() (backup.BackupStorage, error) {
	if s3 {
		logger.Info("configuring S3 backup storage")
		return getS3BackupStorage()
	}
	logger.Info("configuring filesystem backup storage")
	return backup.NewFileSystemBackupStorage(path, logger.WithName("file-system-storage")), nil
}

func getS3BackupStorage() (backup.BackupStorage, error) {
	opts := []backup.S3BackupStorageOpt{
		backup.WithTLS(s3TLS),
		backup.WithCACertPath(s3CACertPath),
		backup.WithRegion(s3Region),
		backup.WithPrefix(s3Prefix),
	}
	return backup.NewS3BackupStorage(
		path,
		s3Bucket,
		s3Endpoint,
		logger.WithName("s3-storage"),
		opts...,
	)
}

func readTargetFile() (string, error) {
	bytes, err := os.ReadFile(targetFilePath)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func compressFile(f string, compression mariadbv1alpha1.CompressAlgorithm) error {

	originalFile, err := os.Open(f)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	tmpf := f + ".tmp"
	compressedFile, err := os.Create(tmpf)
	if err != nil {
		return err
	}
	defer compressedFile.Close()

	switch compression {

	case mariadbv1alpha1.CompressGzip:
		writer := gzip.NewWriter(compressedFile)
		defer writer.Close()
		_, err = io.Copy(writer, originalFile)
		if err != nil {
			return (err)
		}
		writer.Flush()

	case mariadbv1alpha1.CompressZlib:
		writer := zlib.NewWriter(compressedFile)
		defer writer.Close()
		_, err = io.Copy(writer, originalFile)
		if err != nil {
			return (err)
		}
		writer.Flush()

	default:
		err = os.Remove(tmpf)
		if err != nil {
			return (err)
		}
		return errors.New("Unknown compression algorithm")
	}

	err = os.Remove(f)
	if err != nil {
		return (err)
	}

	err = os.Rename(tmpf, f)
	if err != nil {
		return err
	}

	return nil
}
