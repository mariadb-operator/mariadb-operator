package environment

import (
	"context"
	"errors"
	"strings"

	"github.com/sethvargo/go-envconfig"
)

type Environment struct {
	MariadbOperatorName      string `env:"MARIADB_OPERATOR_NAME,required"`
	MariadbOperatorNamespace string `env:"MARIADB_OPERATOR_NAMESPACE,required"`
	MariadbOperatorSAPath    string `env:"MARIADB_OPERATOR_SA_PATH,required"`
	MariadbOperatorImage     string `env:"MARIADB_OPERATOR_IMAGE,required"`
	RelatedMariadbImage      string `env:"RELATED_IMAGE_MARIADB,required"`
	WatchNamespace           string `env:"WATCH_NAMESPACE"`
}

func (e *Environment) WatchNamespaces() ([]string, error) {
	if e.WatchNamespace == "" {
		return nil, errors.New("WATCH_NAMESPACE environment variable not set")
	}
	if strings.Contains(e.WatchNamespace, ",") {
		return strings.Split(e.WatchNamespace, ","), nil
	}
	return []string{e.WatchNamespace}, nil
}

func GetEnvironment(ctx context.Context) (*Environment, error) {
	var env Environment
	if err := envconfig.Process(ctx, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
