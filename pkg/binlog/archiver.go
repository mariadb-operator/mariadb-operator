package binlog

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Archiver struct {
	dataDir string
	env     *environment.PodEnvironment
	client  client.Client
	logger  logr.Logger
}

func NewArchiver(dataDir string, env *environment.PodEnvironment, client *client.Client,
	logger logr.Logger) *Archiver {
	return &Archiver{
		dataDir: dataDir,
		env:     env,
		client:  *client,
		logger:  logger,
	}
}

func (a *Archiver) Start(ctx context.Context) error {
	return nil
}
