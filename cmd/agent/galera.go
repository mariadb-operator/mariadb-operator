package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/filemanager"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/spf13/cobra"
)

var galeraCommand = &cobra.Command{
	Use:   "galera",
	Short: "Galera.",
	Long:  "Galera agent.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Galera agent starting")

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

		apiServer, err := getAPIServer(
			env,
			fileManager,
			k8sClient,
			logger,
		)
		if err != nil {
			logger.Error(err, "Error creating API server")
			os.Exit(1)
		}
		probeServer, err := getProbeServer(env, k8sClient)
		if err != nil {
			logger.Error(err, "Error creating probe server")
			os.Exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		defer cancel()
		errChan := make(chan error, 2)

		var wg sync.WaitGroup
		wg.Add(2)
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
		go func() {
			wg.Wait()
			close(errChan)
		}()

		if err, ok := <-errChan; ok {
			logger.Error(err, "Server error")
			os.Exit(1)
		}
		logger.Info("Galera agent stopped")
	},
}
