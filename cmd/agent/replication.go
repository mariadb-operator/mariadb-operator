package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	replicationhandler "github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/handler/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/server"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/spf13/cobra"
)

var replicationCommand = &cobra.Command{
	Use:   "replication",
	Short: "Replication.",
	Long:  "Replication agent.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Replication agent starting")

		env, err := environment.GetPodEnv(context.Background())
		if err != nil {
			logger.Error(err, "Error getting environment variables")
			os.Exit(1)
		}

		probeServer, err := getReplicationProbeServer(env, logger.WithName("probe"))
		if err != nil {
			logger.Error(err, "Error creating probe server")
			os.Exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		defer cancel()

		if err := probeServer.Start(ctx); err != nil {
			logger.Error(err, "Error starting probe server")
			os.Exit(1)
		}

		logger.Info("Replication agent stopped")
	},
}

func getReplicationProbeServer(env *environment.PodEnvironment, logger logr.Logger) (*server.Server, error) {
	handler := replicationhandler.NewReplicationProbe(
		env,
		mdbhttp.NewResponseWriter(&logger),
		&logger,
	)
	router := router.NewProbeRouter(
		handler,
		logger,
	)

	server, err := server.NewServer(
		probeAddr,
		router,
		&logger,
	)
	if err != nil {
		return nil, err
	}
	return server, nil
}
