package pitr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/azure"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/binlog"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	mariadbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	mariadbminio "github.com/mariadb-operator/mariadb-operator/v26/pkg/minio"
	mariadbrepl "github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

var (
	logger = ctrl.Log

	path           string
	targetFilePath string

	startGtidRaw  string
	targetTimeRaw string
	strictMode    bool

	s3           bool
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	s3TLS        bool
	s3CACertPath string
	s3Prefix     string

	abs           bool
	absContainer  string
	absServiceURL string
	absTLS        bool
	absCACertPath string
	absPrefix     string

	compression string

	pullBackoff = wait.Backoff{
		Steps:    10,
		Duration: 1 * time.Second,
	}
)

func init() {
	RootCmd.Flags().StringVar(&path, "path", "/binlogs", "Directory path where the binary log files will be pulled.")
	RootCmd.Flags().StringVar(&targetFilePath, "target-file-path", "/binlogs/0-binlog-target.txt",
		"Path to a file that contains the names of the binlog target files.")

	RootCmd.Flags().StringVar(&startGtidRaw, "start-gtid", "",
		"Initial GTID (global transaction ID) from which the binlogs will be pulled.")
	RootCmd.Flags().StringVar(&targetTimeRaw, "target-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the recovery point-in-time.")
	RootCmd.Flags().BoolVar(&strictMode, "strict-mode", false,
		"Strict mode that controls the behavior when a point-in-time restoration cannot reach the exact target time."+
			"When enabled, returns an error and avoids replaying binary logs if target time is not reached."+
			"When disabled (default), replays available binary logs until the last recoverable time.")

	RootCmd.Flags().BoolVar(&s3, "s3", false, "Enable S3 binlog storage.")
	RootCmd.Flags().StringVar(&s3Bucket, "s3-bucket", "binlogs", "Name of the bucket to store binary logs.")
	RootCmd.Flags().StringVar(&s3Prefix, "s3-prefix", "", "S3 bucket prefix name to use.")
	RootCmd.Flags().StringVar(&s3Endpoint, "s3-endpoint", "s3.amazonaws.com", "S3 API endpoint without scheme.")
	RootCmd.Flags().StringVar(&s3Region, "s3-region", "us-east-1", "S3 region name to use.")
	RootCmd.Flags().BoolVar(&s3TLS, "s3-tls", false, "Enable S3 TLS connections.")
	RootCmd.Flags().StringVar(&s3CACertPath, "s3-ca-cert-path", "", "Path to the CA to be trusted when connecting to S3.")

	RootCmd.PersistentFlags().BoolVar(&abs, "abs", false, "Enable Azure Blob backup storage.")
	RootCmd.PersistentFlags().StringVar(&absContainer, "abs-container", "backups", "Name of the container to store backups.")
	RootCmd.PersistentFlags().StringVar(&absServiceURL, "abs-service-url", "", "Full abs service url to use: http(s)://<account>.blob.core.windows.net/.")
	RootCmd.PersistentFlags().BoolVar(&absTLS, "abs-tls", false, "Enable Azure Blob Storage TLS connections.")
	RootCmd.PersistentFlags().StringVar(&absCACertPath, "abs-ca-cert-path", "", "Path to the CA to be trusted when connecting to ABS.")
	RootCmd.PersistentFlags().StringVar(&absPrefix, "abs-prefix", "", "ABS container prefix to use.")

	RootCmd.Flags().StringVar(&compression, "compression", string(mariadbv1alpha1.CompressNone),
		"Compression algorithm: none, gzip or bzip2.")
}

var RootCmd = &cobra.Command{
	Use:   "pitr",
	Short: "PITR.",
	Long:  `Pulls and decompresses binary logs from object storage to enable point-in-time recovery using mariadb-binlog.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("Error setting up logger: %v\n", err)
			os.Exit(1)
		}
		startGtid, err := mariadbrepl.ParseGtid(startGtidRaw)
		if err != nil {
			logger.Error(err, "Error parsing start GTID", "gtid", startGtidRaw)
			os.Exit(1)
		}
		targetTime, err := time.Parse(time.RFC3339, targetTimeRaw)
		if err != nil {
			logger.Error(err, "Error parsing target time", "time", targetTimeRaw)
			os.Exit(1)
		}
		calg, err := getCompressionAlgorithm()
		if err != nil {
			logger.Error(err, "Error getting compression algorithm", "compression", compression)
			os.Exit(1)
		}
		logger.Info("Starting point-in-time recovery")

		ctx, cancel := newContext()
		defer cancel()

		storageClient, err := getStorageClient()
		if err != nil {
			logger.Error(err, "Error getting S3 client")
			os.Exit(1)
		}

		logger.Info("Getting binlog index from object storage")
		binlogIndex, err := getBinlogIndex(ctx, storageClient)
		if err != nil {
			logger.Error(err, "Error getting binlog index")
			os.Exit(1)
		}

		logger.Info("Building binlog timeline")
		binlogMetas, err := binlogIndex.BuildTimeline(startGtid, targetTime, strictMode, logger.WithName("binlog-timeline"))
		if err != nil {
			logger.Error(err, "Error getting binlog timeline")
			os.Exit(1)
		}
		binlogPath := getBinlogTimeline(binlogMetas)
		logger.Info("Got binlog timeline", "path", binlogPath)

		logger.Info("Pulling binlogs into staging area", "staging-path", path, "compression", calg)
		if err := pullBinlogs(ctx, binlogPath, calg, storageClient, logger.WithName("storage")); err != nil {
			logger.Error(err, "Error pulling binlogs")
			os.Exit(1)
		}

		logger.Info("Writing target file", "file-path", targetFilePath)
		if err := writeTargetFile(binlogPath); err != nil {
			logger.Error(err, "Error writing target file")
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

func getStorageClient() (interfaces.BlobStorage, error) {
	if abs {
		return getABSClient()
	}
	if s3 {
		return getS3Client()
	}

	return nil, fmt.Errorf("error getting a storage client, none configured. Either abs or s3 must be configured")
}

// getABSClient retrieves an Azure Blob Storage client
// @WARN: should not be used directly, see `getStorageClient`
func getABSClient() (*azure.AzBlobClient, error) {
	logger.Info("configuring ABS backup storage")
	opts := []azure.AzBlobOpt{
		azure.WithTLSEnabled(absTLS),
		azure.WithTLSCACertPath(absCACertPath),
		azure.WithPrefix(absPrefix),
		azure.WithAllowNestedPrefixes(true),
	}
	if accountKey := os.Getenv(builder.ABSStorageAccountKey); accountKey != "" {
		opts = append(opts, azure.WithAccountKey(accountKey))
	}

	if accountName := os.Getenv(builder.ABSStorageAccountName); accountName != "" {
		opts = append(opts, azure.WithAccountName(accountName))
	}

	return azure.NewAzBlobClient(path, absContainer, absServiceURL, opts...)
}

func getS3Client() (*mariadbminio.Client, error) {
	minioOpts := []mariadbminio.MinioOpt{
		mariadbminio.WithTLS(s3TLS),
		mariadbminio.WithCACertPath(s3CACertPath),
		mariadbminio.WithRegion(s3Region),
		mariadbminio.WithPrefix(s3Prefix),
		mariadbminio.WithAllowNestedPrefixes(true),
	}

	if ssecKey := os.Getenv(builder.S3SSECCustomerKey); ssecKey != "" {
		logger.Info("configuring S3 SSE-C encryption")
		minioOpts = append(minioOpts, mariadbminio.WithSSECCustomerKey(ssecKey))
	}

	client, err := mariadbminio.NewMinioClient(path, s3Bucket, s3Endpoint, minioOpts...)
	if err != nil {
		return nil, fmt.Errorf("error getting S3 client: %v", err)
	}
	return client, nil
}

func getBinlogIndex(ctx context.Context, storageClient interfaces.BlobStorage) (*binlog.BinlogIndex, error) {
	exists, err := storageClient.Exists(ctx, binlog.BinlogIndexName)
	if err != nil {
		return nil, fmt.Errorf("error checking if binlog index exists: %v", err)
	}
	if !exists {
		return nil, errors.New("binlog index not found")
	}

	indexReader, err := storageClient.GetObjectWithOptions(ctx, binlog.BinlogIndexName)
	if err != nil {
		return nil, fmt.Errorf("error getting binlog index: %v", err)
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

func getBinlogTimeline(binlogMetas []binlog.BinlogMetadata) []string {
	binlogPath := make([]string, len(binlogMetas))
	for i, binlogMeta := range binlogMetas {
		binlogPath[i] = binlogMeta.ObjectStoragePath()
	}
	return binlogPath
}

func pullBinlogs(ctx context.Context, binlogPath []string, calg mariadbv1alpha1.CompressAlgorithm, storageClient interfaces.BlobStorage,
	logger logr.Logger) error {
	for _, binlog := range binlogPath {
		if err := pullBinlog(ctx, binlog, calg, storageClient, logger); err != nil {
			return fmt.Errorf("error pulling binlog %s: %v", binlog, err)
		}
	}
	return nil
}

func pullBinlog(ctx context.Context, binlog string, calg mariadbv1alpha1.CompressAlgorithm, storageClient interfaces.BlobStorage,
	logger logr.Logger) error {
	logger.Info("Pulling binlog", "binlog", binlog)

	ext, err := calg.Extension()
	if err != nil {
		return fmt.Errorf("error getting extension for compression algorithm %s: %v", calg, err)
	}
	compressedFileName := binlog
	if ext != "" {
		compressedFileName = fmt.Sprintf("%s.%s", binlog, ext)
	}

	exists, err := storageClient.Exists(ctx, compressedFileName)
	if err != nil {
		return fmt.Errorf("error determining if %s exists: %v", compressedFileName, err)
	}
	if !exists {
		return fmt.Errorf("binlog file %s not found", compressedFileName)
	}

	pullIsRetriable := func(err error) bool {
		if ctx.Err() != nil {
			return false
		}
		return err != nil
	}
	var compressedFile io.ReadCloser
	if err := retry.OnError(pullBackoff, pullIsRetriable, func() error {
		compressedFile, err = storageClient.GetObjectWithOptions(ctx, compressedFileName)
		return err
	}); err != nil {
		return fmt.Errorf("error pulling binlog file %s: %v", compressedFileName, err)
	}
	defer compressedFile.Close()

	plainFileName := filepath.Join(path, binlog)
	plainFileDir := filepath.Dir(plainFileName)
	if err := os.MkdirAll(plainFileDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating binlog dir %s: %v", plainFileDir, err)
	}
	plainFile, err := os.Create(plainFileName)
	if err != nil {
		return fmt.Errorf("error creating binlog file %s: %v", plainFileName, err)
	}
	defer plainFile.Close()

	compressor, err := mariadbcompression.NewCompressor(calg)
	if err != nil {
		return fmt.Errorf("error getting compressor: %v", err)
	}

	if calg != "" && calg != mariadbv1alpha1.CompressNone {
		logger.Info(
			"Decompressing binlog",
			"compressed-file", compressedFileName,
			"decompressed-file", plainFileName,
			"compression", calg,
		)
	}
	if err := compressor.Decompress(ctx, plainFile, compressedFile); err != nil {
		return fmt.Errorf("error decompressing file %s into %s: %v", compressedFileName, plainFileName, err)
	}
	return nil
}

func getCompressionAlgorithm() (mariadbv1alpha1.CompressAlgorithm, error) {
	calg := mariadbv1alpha1.CompressAlgorithm(compression)
	if err := calg.Validate(); err != nil {
		return "", fmt.Errorf("compression algorithm not supported: %v", err)
	}
	return calg, nil
}

func writeTargetFile(binlogPath []string) error {
	targetBinlogsWithLocalPath := make([]string, len(binlogPath))
	for i, binlog := range binlogPath {
		targetBinlogsWithLocalPath[i] = filepath.Join(path, binlog)
	}
	return os.WriteFile(targetFilePath, []byte(strings.Join(targetBinlogsWithLocalPath, " ")), 0777)
}
