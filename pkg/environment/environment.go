package environment

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sethvargo/go-envconfig"
)

type OperatorEnv struct {
	MariadbOperatorName          string `env:"MARIADB_OPERATOR_NAME,required"`
	MariadbOperatorNamespace     string `env:"MARIADB_OPERATOR_NAMESPACE,required"`
	MariadbOperatorSAPath        string `env:"MARIADB_OPERATOR_SA_PATH,required"`
	MariadbOperatorImage         string `env:"MARIADB_OPERATOR_IMAGE,required"`
	RelatedMariadbImage          string `env:"RELATED_IMAGE_MARIADB,required"`
	RelatedMaxscaleImage         string `env:"RELATED_IMAGE_MAXSCALE,required"`
	RelatedExporterImage         string `env:"RELATED_IMAGE_EXPORTER,required"`
	RelatedExporterMaxscaleImage string `env:"RELATED_IMAGE_EXPORTER_MAXSCALE,required"`
	MariadbGaleraLibPath         string `env:"MARIADB_GALERA_LIB_PATH,required"`
	MariadbEntrypointVersion     string `env:"MARIADB_ENTRYPOINT_VERSION,required"`
	WatchNamespace               string `env:"WATCH_NAMESPACE"`
}

func (e *OperatorEnv) WatchNamespaces() ([]string, error) {
	if e.WatchNamespace == "" {
		return nil, errors.New("WATCH_NAMESPACE environment variable not set")
	}
	if strings.Contains(e.WatchNamespace, ",") {
		return strings.Split(e.WatchNamespace, ","), nil
	}
	return []string{e.WatchNamespace}, nil
}

func (e *OperatorEnv) CurrentNamespaceOnly() (bool, error) {
	if e.WatchNamespace == "" {
		return false, nil
	}
	watchNamespaces, err := e.WatchNamespaces()
	if err != nil {
		return false, fmt.Errorf("error getting namespaces to watch: %v", err)
	}

	if len(watchNamespaces) != 1 {
		return false, nil
	}
	return watchNamespaces[0] == e.MariadbOperatorNamespace, nil
}

func GetOperatorEnv(ctx context.Context) (*OperatorEnv, error) {
	var env OperatorEnv
	if err := envconfig.Process(ctx, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

type PodEnvironment struct {
	ClusterName         string `env:"CLUSTER_NAME,required"`
	PodName             string `env:"POD_NAME,required"`
	PodNamespace        string `env:"POD_NAMESPACE,required"`
	PodIP               string `env:"POD_IP,required"`
	MariadbName         string `env:"MARIADB_NAME,required"`
	MariadbRootPassword string `env:"MARIADB_ROOT_PASSWORD,required"`
	MariadbPort         string `env:"MYSQL_TCP_PORT,required"`
	TLSEnabled          string `env:"TLS_ENABLED"`
	TLSCACertPath       string `env:"TLS_CA_CERT_PATH"`
	TLSServerCertPath   string `env:"TLS_SERVER_CERT_PATH"`
	TLSServerKeyPath    string `env:"TLS_SERVER_KEY_PATH"`
	TLSClientCertPath   string `env:"TLS_CLIENT_CERT_PATH"`
	TLSClientKeyPath    string `env:"TLS_CLIENT_KEY_PATH"`
}

func (e *PodEnvironment) Port() (int32, error) {
	port, err := strconv.Atoi(e.MariadbPort)
	if err != nil {
		return 0, err
	}
	return int32(port), nil
}

func (e *PodEnvironment) IsTLSEnabled() (bool, error) {
	if e.TLSEnabled == "" {
		return false, nil
	}
	return strconv.ParseBool(e.TLSEnabled)
}

func GetPodEnv(ctx context.Context) (*PodEnvironment, error) {
	var env PodEnvironment
	if err := envconfig.Process(ctx, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
