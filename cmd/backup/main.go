package backup

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger                = ctrl.Log
	path                  string
	targetFilePath        string
	s3                    bool
	s3Bucket              string
	s3Endpoint            string
	s3AccessKeyIdPath     string
	s3SecretAccessKeyPath string
	maxRetention          time.Duration
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
	RootCmd.PersistentFlags().StringVar(&s3Endpoint, "s3-endpoint", "s3.amazonaws.com", "S3 endpoint URL.")
	RootCmd.PersistentFlags().StringVar(&s3AccessKeyIdPath, "s3-access-key-id-path", "/s3/access-key-id",
		"File path containing the S3 key identifier to authenticate.")
	RootCmd.PersistentFlags().StringVar(&s3SecretAccessKeyPath, "s3-secret-access-key-path", "/s3/secret-access-key",
		"File path containing the S3 secret key to authenticate.")

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
		if err := setupLogger(cmd); err != nil {
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

		logger.Info("pushing target backup", "file", backupTargetFile)
		if err := backupStorage.Push(ctx, backupTargetFile); err != nil {
			logger.Error(err, "error pushing target backup", "file", backupTargetFile)
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
		accessKeyId, secretAccessKey, err := readS3Credentials()
		if err != nil {
			return nil, err
		}
		return backup.NewS3BackupStorage(
			path,
			s3Bucket,
			s3Endpoint,
			accessKeyId,
			secretAccessKey,
			logger.WithName("s3-storage"),
		)
	}
	logger.Info("configuring filesystem backup storage")
	return backup.NewFileSystemBackupStorage(path, logger.WithName("file-system-storage")), nil
}

func readS3Credentials() (accessKeyId string, secretAccessKey string, err error) {
	accessKeyId, err = readEnvFallbackToFile("S3_ACCESS_KEY_ID", s3AccessKeyIdPath)
	if err != nil {
		return "", "", fmt.Errorf("error reading access key id: %v", err)
	}
	secretAccessKey, err = readEnvFallbackToFile("S3_SECRET_ACCESS_KEY", s3AccessKeyIdPath)
	if err != nil {
		return "", "", fmt.Errorf("error reading secret access key: %v", err)
	}
	return
}

func readEnvFallbackToFile(env, filePath string) (string, error) {
	envVal := os.Getenv(env)
	if envVal != "" {
		return envVal, nil
	}
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil
	}
	return string(bytes), nil
}

func readTargetFile() (string, error) {
	bytes, err := os.ReadFile(targetFilePath)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
