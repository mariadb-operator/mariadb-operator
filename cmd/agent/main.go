package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/handler"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/router"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/agent/server"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/state"
	kubeauth "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/auth"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme    = runtime.NewScheme()
	logger    = ctrl.Log
	addr      string
	configDir string
	stateDir  string

	compressLevel           int
	rateLimitRequests       int
	rateLimitDuration       time.Duration
	gracefulShutdownTimeout time.Duration

	kubernetesAuth             bool
	kubernetesTrustedName      string
	kubernetesTrustedNamespace string

	basicAuth             bool
	basicAuthUsername     string
	basicAuthPasswordPath string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))

	RootCmd.Flags().StringVar(&addr, "addr", ":5555", "The address that the HTTP server binds to")
	RootCmd.Flags().StringVar(&configDir, "config-dir", "/etc/mysql/mariadb.conf.d", "The directory that contains MariaDB configuration files")
	RootCmd.Flags().StringVar(&stateDir, "state-dir", "/var/lib/mysql", "The directory that contains MariaDB state files")

	RootCmd.Flags().IntVar(&compressLevel, "compress-level", 5, "HTTP compression level")
	RootCmd.Flags().IntVar(&rateLimitRequests, "rate-limit-requests", 0, "Number of requests to be used as rate limit")
	RootCmd.Flags().DurationVar(&rateLimitDuration, "rate-limit-duration", 0, "Duration to be used as rate limit")
	RootCmd.Flags().DurationVar(&gracefulShutdownTimeout, "graceful-shutdown-timeout", 5*time.Second, "Timeout to gracefully terminate "+
		"in-flight requests")

	RootCmd.Flags().BoolVar(&kubernetesAuth, "kubernetes-auth", false, "Enable Kubernetes authentication via the TokenReview API")
	RootCmd.Flags().StringVar(&kubernetesTrustedName, "kubernetes-trusted-name", "", "Trusted Kubernetes ServiceAccount name to be verified")
	RootCmd.Flags().StringVar(&kubernetesTrustedNamespace, "kubernetes-trusted-namespace", "", "Trusted Kubernetes ServiceAccount "+
		"namespace to be verified")

	RootCmd.Flags().BoolVar(&basicAuth, "basic-auth", false, "Enable basic authentication")
	RootCmd.Flags().StringVar(&basicAuthUsername, "basic-auth-username", "", "Basic authentication username")
	RootCmd.Flags().StringVar(&basicAuthPasswordPath, "basic-auth-password-path", "", "Basic authentication password path")
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
		logger.Info("Starting agent")

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
		state := state.NewState(stateDir)
		k8sClient, err := getK8sClient()
		if err != nil {
			logger.Error(err, "Error getting Kubernetes client")
			os.Exit(1)
		}

		mariadbKey := types.NamespacedName{
			Name:      env.MariadbName,
			Namespace: env.PodNamespace,
		}
		handlerLogger := logger.WithName("handler")
		handler := handler.NewHandler(
			mariadbKey,
			k8sClient,
			fileManager,
			state,
			&handlerLogger,
		)

		routerOpts := []router.Option{
			router.WithCompressLevel(compressLevel),
			router.WithRateLimit(rateLimitRequests, rateLimitDuration),
		}
		if kubernetesAuth && kubernetesTrustedName != "" && kubernetesTrustedNamespace != "" {
			logger.Info("Configuring Kubernetes authentication")

			routerOpts = append(routerOpts, router.WithKubernetesAuth(
				kubernetesAuth,
				&kubeauth.Trusted{
					ServiceAccountName:      kubernetesTrustedName,
					ServiceAccountNamespace: kubernetesTrustedNamespace,
				},
			))
		} else if basicAuth && basicAuthUsername != "" && basicAuthPasswordPath != "" {
			logger.Info("Configuring basic authentication")

			basicAuthPassword, err := os.ReadFile(basicAuthPasswordPath)
			if err != nil {
				logger.Error(err, "Error reading basic-auth password")
				os.Exit(1)
			}
			routerOpts = append(routerOpts, router.WithBasicAuth(
				basicAuth,
				basicAuthUsername,
				string(basicAuthPassword),
			))
		}
		router := router.NewRouter(
			handler,
			k8sClient,
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
			logger.Error(err, "Server error")
			os.Exit(1)
		}
	},
}

func getK8sClient() (client.Client, error) {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting REST config: %v", err)
	}
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client: %v", err)
	}
	return k8sClient, nil
}
