package agent

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/go-logr/logr"
	replicationhandler "github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/handler/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/server"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/binlog"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		fileManager, err := filemanager.NewFileManager(configDir, stateDir)
		if err != nil {
			logger.Error(err, "Error creating file manager")
			os.Exit(1)
		}
		k8sClient, err := getK8sClient()
		if err != nil {
			logger.Error(err, "Error getting Kubernetes client")
			os.Exit(1)
		}

		apiLogger := logger.WithName("api")
		apiHandler := replicationhandler.NewReplicationHandler(
			fileManager,
			mdbhttp.NewResponseWriter(&apiLogger),
			&apiLogger,
		)
		apiServer, err := getAPIServer(
			apiHandler,
			env,
			k8sClient,
			apiLogger,
		)
		if err != nil {
			logger.Error(err, "Error creating API server")
			os.Exit(1)
		}

		probeServer, err := getReplicationProbeServer(env, k8sClient, logger.WithName("probe"))
		if err != nil {
			logger.Error(err, "Error creating probe server")
			os.Exit(1)
		}

		ctx, cancel := newContext()
		defer cancel()

		numGoroutines := 2
		if binaryLogArchival {
			numGoroutines++
		}
		errChan := make(chan error, numGoroutines)
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		go func() {
			defer wg.Done()

			if err := apiServer.Start(ctx); err != nil {
				errChan <- fmt.Errorf("error starting API server: %v", err)
			}
		}()
		go func() {
			defer wg.Done()

			if err := probeServer.Start(ctx); err != nil {
				errChan <- fmt.Errorf("error starting probe server: %v", err)
			}
		}()
		if binaryLogArchival {
			archiver := binlog.NewArchiver(
				stateDir,
				env,
				k8sClient,
				logger.WithName("binlog-archival"),
			)
			go func() {
				defer wg.Done()

				if err := archiver.Start(ctx); err != nil {
					errChan <- fmt.Errorf("error starting binlog archiver: %v", err)
				}
			}()
		}
		go func() {
			wg.Wait()
			close(errChan)
		}()

		if err, ok := <-errChan; ok {
			logger.Error(err, "Agent error")
			os.Exit(1)
		}
		logger.Info("Replication agent stopped")
	},
}

func getReplicationProbeServer(env *environment.PodEnvironment, k8sClient client.Client, logger logr.Logger) (*server.Server, error) {
	handler := replicationhandler.NewReplicationProbe(
		env,
		k8sClient,
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
