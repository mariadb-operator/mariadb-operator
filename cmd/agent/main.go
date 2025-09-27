package agent

import (
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/router"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/server"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	kubeauth "github.com/mariadb-operator/mariadb-operator/v25/pkg/kubernetes/auth"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme    = runtime.NewScheme()
	logger    = ctrl.Log
	addr      string
	probeAddr string
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

	RootCmd.Flags().StringVar(&addr, "addr", ":5555", "The address that the HTTP(s) API server binds to")
	RootCmd.Flags().StringVar(&probeAddr, "probe-addr", ":5566", "The address that the HTTP probe server binds to")

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

	RootCmd.AddCommand(galeraCommand)
}

var RootCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent.",
	Long:  "Sidecar agent that co-operates with mariadb-operator.",
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

func getAPIServer(apiHandler router.RouteHandler, env *environment.PodEnvironment, k8sClient client.Client,
	logger logr.Logger) (*server.Server, error) {
	routerOpts, err := getRouterOpts(logger)
	if err != nil {
		return nil, err
	}
	router := router.NewRouter(
		apiHandler,
		k8sClient,
		logger,
		routerOpts...,
	)

	serverOpts, err := getServerOpts(env)
	if err != nil {
		return nil, err
	}
	server, err := server.NewServer(
		addr,
		router,
		&logger,
		serverOpts...,
	)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func getRouterOpts(logger logr.Logger) ([]router.Option, error) {
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
			return nil, err
		}
		routerOpts = append(routerOpts, router.WithBasicAuth(
			basicAuth,
			basicAuthUsername,
			string(basicAuthPassword),
		))
	}
	return routerOpts, nil
}

func getServerOpts(env *environment.PodEnvironment) ([]server.Option, error) {
	serverOpts := []server.Option{
		server.WithGracefulShutdownTimeout(gracefulShutdownTimeout),
	}
	isTLSEnabled, err := env.IsTLSEnabled()
	if err != nil {
		return nil, err
	}
	if isTLSEnabled {
		serverOpts = append(serverOpts, []server.Option{
			server.WithTLSEnabled(isTLSEnabled),
			server.WithTLSCAPath(env.TLSCACertPath),
			server.WithTLSCertPath(env.TLSServerCertPath),
			server.WithTLSKeyPath(env.TLSServerKeyPath),
		}...)
	}
	return serverOpts, nil
}
