package environment

import (
	"context"

	"github.com/sethvargo/go-envconfig"
)

type Environment struct {
	MariadbOperatorName      string `env:"MARIADB_OPERATOR_NAME,required"`
	MariadbOperatorNamespace string `env:"MARIADB_OPERATOR_NAMESPACE,required"`
	MariadbOperatorSAPath    string `env:"MARIADB_OPERATOR_SA_PATH,required"`
	RelatedMariadbImage      string `env:"RELATED_IMAGE_MARIADB,required"`
}

func GetEnvironment(ctx context.Context) (*Environment, error) {
	var env Environment
	if err := envconfig.Process(ctx, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
