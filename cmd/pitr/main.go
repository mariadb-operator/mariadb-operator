package pitr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/binlog"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v25/pkg/minio"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log

	path           string
	targetFilePath string

	startGtidRaw  string
	targetTimeRaw string

	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	s3TLS        bool
	s3CACertPath string
	s3Prefix     string
)

func init() {
	RootCmd.Flags().StringVar(&path, "path", "/binlogs", "Directory path where the binary log files will be pulled.")
	RootCmd.Flags().StringVar(&targetFilePath, "target-file-path", "/binlogs/0-binlog-target.txt",
		"Path to a file that contains the names of the binlog target files.")

	RootCmd.Flags().StringVar(&startGtidRaw, "start-gtid", "",
		"Initial GTID (global transaction ID) from which the binlogs will be pulled.")
	RootCmd.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the recovery point-in-time.")

	RootCmd.Flags().StringVar(&s3Bucket, "s3-bucket", "binlogs", "Name of the bucket to store binary logs.")
	RootCmd.Flags().StringVar(&s3Endpoint, "s3-endpoint", "s3.amazonaws.com", "S3 API endpoint without scheme.")
	RootCmd.Flags().StringVar(&s3Region, "s3-region", "us-east-1", "S3 region name to use.")
	RootCmd.Flags().BoolVar(&s3TLS, "s3-tls", false, "Enable S3 TLS connections.")
	RootCmd.Flags().StringVar(&s3CACertPath, "s3-ca-cert-path", "", "Path to the CA to be trusted when connecting to S3.")
	RootCmd.Flags().StringVar(&s3Prefix, "s3-prefix", "", "S3 bucket prefix name to use.")
}

var RootCmd = &cobra.Command{
	Use:   "pitr",
	Short: "PITR.",
	Long:  `Pulls binary logs from object storage to enable point-in-time recovery using mariadb-binlog.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting ppoint-in-time recovery")

		ctx, cancel := newContext()
		defer cancel()

		s3Client, err := getS3Client()
		if err != nil {
			logger.Error(err, "error getting S3 client")
			os.Exit(1)
		}

		_, err = getBinlogIndex(ctx, s3Client)
		if err != nil {
			logger.Error(err, "error getting binlog index")
			os.Exit(1)
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

func getS3Client() (*mariadbminio.Client, error) {
	minioOpts := []mariadbminio.MinioOpt{
		mariadbminio.WithTLS(s3TLS),
		mariadbminio.WithCACertPath(s3CACertPath),
		mariadbminio.WithRegion(s3Region),
		mariadbminio.WithPrefix(s3Prefix),
		mariadbminio.WithAllowNestedPrefixes(true),
	}
	// TODO: support for SSEC based on environment
	client, err := mariadbminio.NewMinioClient(path, s3Bucket, s3Endpoint, minioOpts...)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 client: %v", err)
	}
	return client, nil
}

func getBinlogIndex(ctx context.Context, s3Client *mariadbminio.Client) (*binlog.BinlogIndex, error) {
	exists, err := s3Client.Exists(ctx, binlog.BinlogIndexName)
	if err != nil {
		return nil, fmt.Errorf("error checking if binlog index exists: %v", err)
	}
	if !exists {
		return nil, errors.New("binlog index not found")
	}

	indexReader, err := s3Client.GetObjectWithOptions(ctx, binlog.BinlogIndexName)
	if err != nil {
		logger.Error(err, "error binlog index")
		os.Exit(1)
	}
	defer indexReader.Close()

	bytes, err := io.ReadAll(indexReader)
	if err != nil {
		return nil, fmt.Errorf("error reading binlog index: %w", err)
	}
	var bi binlog.BinlogIndex
	if err := yaml.Unmarshal(bytes, &bi); err != nil {
		return nil, fmt.Errorf("error unmarshaling binlog index: %v", err)
	}
	return &bi, nil
}
