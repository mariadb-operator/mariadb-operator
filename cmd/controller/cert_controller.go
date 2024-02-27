package main

import (
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/controller"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	caSecretName, caSecretNamespace, caCommonName string
	caValidity                                    time.Duration
	certSecretName, certSecretNamespace           string
	certValidity                                  time.Duration
	lookaheadValidity                             time.Duration
	serviceName, serviceNamespace                 string
	requeueDuration                               time.Duration
)

func init() {
	certControllerCmd.Flags().StringVar(&caSecretName, "ca-secret-name", "mariadb-operator-webhook-ca",
		"Secret to store CA certificate for webhook")
	certControllerCmd.Flags().StringVar(&caSecretNamespace, "ca-secret-namespace", "default",
		"Namespace of the Secret to store the CA certificate for webhook")
	certControllerCmd.Flags().StringVar(&caCommonName, "ca-common-name", "mariadb-operator", "CA certificate common name")
	certControllerCmd.Flags().DurationVar(&caValidity, "ca-validity", 4*365*24*time.Hour, "CA certificate validity")
	certControllerCmd.Flags().StringVar(&certSecretName, "cert-secret-name", "mariadb-operator-webhook-cert",
		"Secret to store the certificate for webhook")
	certControllerCmd.Flags().StringVar(&certSecretNamespace, "cert-secret-namespace", "default",
		"Namespace of the Secret to store the certificate for webhook")
	certControllerCmd.Flags().DurationVar(&certValidity, "cert-validity", 365*24*time.Hour, "Certificate validity")
	certControllerCmd.Flags().DurationVar(&lookaheadValidity, "lookahead-validity", 90*24*time.Hour,
		"Lookahead validity used to determine whether a certificate is valid or not")
	certControllerCmd.Flags().StringVar(&serviceName, "service-name", "mariadb-operator-webhook", "Webhook service name")
	certControllerCmd.Flags().StringVar(&serviceNamespace, "service-namespace", "default", "Webhook service namespace")
	certControllerCmd.Flags().DurationVar(&requeueDuration, "requeue-duration", time.Minute*5,
		"Time duration between reconciling webhook config for new certs")
}

var certControllerCmd = &cobra.Command{
	Use:   "cert-controller",
	Short: "MariaDB operator certificate controller.",
	Long:  `Issues and injects certificates for validation and mutation webhooks.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetupLogger(logLevel, logTimeEncoder, logDev)

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: healthAddr,
			LeaderElection:         leaderElect,
			LeaderElectionID:       "cert-controller.mariadb-operator.mariadb.com",
		})
		if err != nil {
			setupLog.Error(err, "Unable to start manager")
			os.Exit(1)
		}

		webhookConfigReconciler := controller.NewWebhookConfigReconciler(
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetEventRecorderFor("webhook-config"),
			mgr.Elected(),
			types.NamespacedName{
				Name:      caSecretName,
				Namespace: caSecretNamespace,
			},
			caCommonName,
			caValidity,
			types.NamespacedName{
				Name:      certSecretName,
				Namespace: certSecretNamespace,
			},
			certValidity,
			lookaheadValidity,
			types.NamespacedName{
				Name:      serviceName,
				Namespace: serviceNamespace,
			},
			requeueDuration,
		)
		if err = webhookConfigReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "webhookconfiguration")
			os.Exit(1)
		}

		handler := webhookConfigReconciler.ReadyHandler(setupLog)
		if err := mgr.AddReadyzCheck("webhook-inject", handler); err != nil {
			setupLog.Error(err, "Unable to add webhook readyz check")
			os.Exit(1)
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Problem running manager")
			os.Exit(1)
		}
	},
}
