package environment

import (
	"context"
	"errors"
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
	MariadbGaleraInitImage       string `env:"MARIADB_GALERA_INIT_IMAGE,required"`
	MariadbGaleraAgentImage      string `env:"MARIADB_GALERA_AGENT_IMAGE,required"`
	MariadbGaleraLibPath         string `env:"MARIADB_GALERA_LIB_PATH,required"`
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
}

func (e *PodEnvironment) Port() (int32, error) {
	port, err := strconv.Atoi(e.MariadbPort)
	if err != nil {
		return 0, err
	}
	return int32(port), nil
}

func GetPodEnv(ctx context.Context) (*PodEnvironment, error) {
	var env PodEnvironment
	if err := envconfig.Process(ctx, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
