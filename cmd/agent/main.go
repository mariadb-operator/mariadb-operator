package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/handler"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/router"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/server"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	kubeauth "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/auth"
	kubeclientset "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/clientset"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger    = ctrl.Log
	addr      string
	configDir string
	stateDir  string

	compressLevel              int
	rateLimitRequests          int
	rateLimitDuration          time.Duration
	kubernetesAuth             bool
	kubernetesTrustedName      string
	kubernetesTrustedNamespace string
	recoveryTimeout            time.Duration
	gracefulShutdownTimeout    time.Duration
)

func init() {
	RootCmd.Flags().StringVar(&addr, "addr", ":5555", "The address that the HTTP server binds to")
	RootCmd.Flags().StringVar(&configDir, "config-dir", "/etc/mysql/mariadb.conf.d", "The directory that contains MariaDB configuration files")
	RootCmd.Flags().StringVar(&stateDir, "state-dir", "/var/lib/mysql", "The directory that contains MariaDB state files")

	RootCmd.Flags().IntVar(&compressLevel, "compress-level", 5, "HTTP compression level")
	RootCmd.Flags().IntVar(&rateLimitRequests, "rate-limit-requests", 0, "Number of requests to be used as rate limit")
	RootCmd.Flags().DurationVar(&rateLimitDuration, "rate-limit-duration", 0, "Duration to be used as rate limit")
	RootCmd.Flags().BoolVar(&kubernetesAuth, "kubernetes-auth", false, "Enable Kubernetes authentication via the TokenReview API")
	RootCmd.Flags().StringVar(&kubernetesTrustedName, "kubernetes-trusted-name", "", "Trusted Kubernetes ServiceAccount name to be verified")
	RootCmd.Flags().StringVar(&kubernetesTrustedNamespace, "kubernetes-trusted-namespace", "", "Trusted Kubernetes ServiceAccount "+
		"namespace to be verified")
	RootCmd.Flags().DurationVar(&recoveryTimeout, "recovery-timeout", 1*time.Minute, "Timeout to obtain sequence number "+
		"during the Galera cluster recovery process")
	RootCmd.Flags().DurationVar(&gracefulShutdownTimeout, "graceful-shutdown-timeout", 5*time.Second, "Timeout to gracefully terminate "+
		"in-flight requests")
}

var RootCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent.",
	Long:  `Sidecar agent for Galera that co-operates with mariadb-operator.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting agent")

		clientset, err := kubeclientset.NewClientSet()
		if err != nil {
			logger.Error(err, "Error creating Kubernetes clientset")
			os.Exit(1)
		}

		fileManager, err := filemanager.NewFileManager(configDir, stateDir)
		if err != nil {
			logger.Error(err, "Error creating file manager")
			os.Exit(1)
		}

		handlerLogger := logger.WithName("handler")
		handler := handler.NewHandler(
			fileManager,
			&handlerLogger,
			handler.WithRecoveryTimeout(recoveryTimeout),
		)

		routerOpts := []router.Option{
			router.WithCompressLevel(compressLevel),
			router.WithRateLimit(rateLimitRequests, rateLimitDuration),
		}
		if kubernetesAuth && kubernetesTrustedName != "" && kubernetesTrustedNamespace != "" {
			routerOpts = append(routerOpts, router.WithKubernetesAuth(
				kubernetesAuth,
				&kubeauth.Trusted{
					ServiceAccountName:      kubernetesTrustedName,
					ServiceAccountNamespace: kubernetesTrustedNamespace,
				},
			))
		}
		router := router.NewRouter(
			handler,
			clientset,
			logger,
			routerOpts...,
		)

		serverLogger := logger.WithName("server")
		server := server.NewServer(
			addr,
			router,
			&serverLogger,
			server.WithGracefulShutdownTimeout(gracefulShutdownTimeout),
		)
		if err := server.Start(context.Background()); err != nil {
			logger.Error(err, "server error")
			os.Exit(1)
		}
	},
}
