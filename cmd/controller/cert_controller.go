package main

import (
	"errors"
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/internal/controller"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	caSecretName, caSecretNamespace, caCommonName string
	caLifetime                                    time.Duration
	certSecretName, certSecretNamespace           string
	certLifetime                                  time.Duration
	renewBeforePercentage                         int32
	serviceName, serviceNamespace                 string
	requeueDuration                               time.Duration
)

func init() {
	certControllerCmd.Flags().StringVar(&caSecretName, "ca-secret-name", "mariadb-operator-webhook-ca",
		"Secret to store CA certificate for webhook")
	certControllerCmd.Flags().StringVar(&caSecretNamespace, "ca-secret-namespace", "default",
		"Namespace of the Secret to store the CA certificate for webhook")
	certControllerCmd.Flags().StringVar(&caCommonName, "ca-common-name", "mariadb-operator", "CA certificate common name")
	certControllerCmd.Flags().DurationVar(&caLifetime, "ca-lifetime", pki.DefaultCALifetime, "CA certificate lifetime")
	certControllerCmd.Flags().StringVar(&certSecretName, "cert-secret-name", "mariadb-operator-webhook-cert",
		"Secret to store the certificate for webhook")
	certControllerCmd.Flags().StringVar(&certSecretNamespace, "cert-secret-namespace", "default",
		"Namespace of the Secret to store the certificate for webhook")
	certControllerCmd.Flags().DurationVar(&certLifetime, "cert-lifetime", pki.DefaultCertLifetime, "Certificate lifetime")
	certControllerCmd.Flags().Int32Var(&renewBeforePercentage, "renew-before-percentage", pki.DefaultRenewBeforePercentage,
		"How long before the certificate expiration should the renewal process be triggered."+
			"For example, if a certificate is valid for 60 minutes, and renew-before-percentage=25, "+
			"cert-controller will begin to attempt to renew the certificate 45 minutes after it was issued"+
			"(i.e. when there are 15 minutes (25%) remaining until the certificate is no longer valid).")
	certControllerCmd.Flags().StringVar(&serviceName, "service-name", "mariadb-operator-webhook", "Webhook service name")
	certControllerCmd.Flags().StringVar(&serviceNamespace, "service-namespace", "default", "Webhook service namespace")
	certControllerCmd.Flags().DurationVar(&requeueDuration, "requeue-duration", 5*time.Minute,
		"Time duration between reconciling webhook config for new certs")
}

var certControllerCmd = &cobra.Command{
	Use:   "cert-controller",
	Short: "MariaDB operator certificate controller.",
	Long:  `Issues and injects certificates for validation and mutation webhooks.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetupLogger(logLevel, logTimeEncoder, logDev)

		if !(renewBeforePercentage >= 10 && renewBeforePercentage <= 90) {
			setupLog.Error(errors.New(
				"renew-before-percentage must be between [10, 90]"),
				"invalid renew-before-percentage",
				"value", renewBeforePercentage,
			)
			os.Exit(1)
		}

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
			caLifetime,
			types.NamespacedName{
				Name:      certSecretName,
				Namespace: certSecretNamespace,
			},
			certLifetime,
			renewBeforePercentage,
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
