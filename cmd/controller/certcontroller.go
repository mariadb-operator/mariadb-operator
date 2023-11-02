/*
Copyright © 2023 Martín Montes <martin11lrx@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package controller

import (
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/controller"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	caSecretName, caSecretNamespace, caCommonName string
	certSecretName, certSecretNamespace           string
	serviceName, serviceNamespace                 string
	requeueInterval                               time.Duration
)

var certcontrollerCmd = &cobra.Command{
	Use:   "certcontroller",
	Short: "MariaDB operator certificate controller.",
	Long:  `Issues and injects certificates to validation and mutation webhooks.`,
	Run: func(cmd *cobra.Command, args []string) {
		setupLogger()

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: healthAddr,
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		validatingWebhookReconciler := controller.NewValidatingWebhookConfigReconciler(
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetEventRecorderFor("validating-webhook-config"),
			types.NamespacedName{
				Name:      caSecretName,
				Namespace: caSecretNamespace,
			},
			caCommonName,
			types.NamespacedName{
				Name:      certSecretName,
				Namespace: certSecretNamespace,
			},
			types.NamespacedName{
				Name:      serviceName,
				Namespace: serviceNamespace,
			},
			requeueInterval,
		)
		if err = validatingWebhookReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Uable to create controller", "controller", "ValidatingWebhookConfig")
			os.Exit(1)
		}
		if err := mgr.AddHealthzCheck("validating-webhook-inject", validatingWebhookReconciler.ReadyCheck); err != nil {
			setupLog.Error(err, "Unable to set up health check")
			os.Exit(1)
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Problem running manager")
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(certcontrollerCmd)
	certcontrollerCmd.Flags().StringVar(&caSecretName, "ca-secret-name", "mariadb-operator-webhook-ca",
		"Secret to store CA certificate for webhook")
	certcontrollerCmd.Flags().StringVar(&caSecretNamespace, "ca-secret-namespace", "default",
		"Namespace of the Secret to store the CA certificate for webhook")
	certcontrollerCmd.Flags().StringVar(&caCommonName, "ca-common-name", "mariadb-operator", "CA certificate common name")
	certcontrollerCmd.Flags().StringVar(&certSecretName, "cert-secret-name", "mariadb-operator-webhook-cert",
		"Secret to store the certificate for webhook")
	certcontrollerCmd.Flags().StringVar(&certSecretNamespace, "cert-secret-namespace", "default",
		"Namespace of the Secret to store the certificate for webhook")
	certcontrollerCmd.Flags().StringVar(&serviceName, "service-name", "mariadb-operator-webhook", "Webhook service name")
	certcontrollerCmd.Flags().StringVar(&serviceNamespace, "service-namespace", "default", "Webhook service namespace")
	certcontrollerCmd.Flags().DurationVar(&requeueInterval, "requeue-interval", time.Minute*5,
		"Time duration between reconciling webhook config for new certs")
}
