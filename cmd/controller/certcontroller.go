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
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	serviceName, serviceNamespace string
	secretName, secretNamespace   string
	webhookConfigRequeueInterval  time.Duration
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
			5*time.Minute,
			"",
			"",
			"",
			"",
		)

		if err = validatingWebhookReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ValidatingWebhookConfig")
			os.Exit(1)
		}

		if err := mgr.AddHealthzCheck("validating-webhook-inject", validatingWebhookReconciler.ReadyCheck); err != nil {
			setupLog.Error(err, "unable to set up health check")
			os.Exit(1)
		}

		setupLog.Info("starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(certcontrollerCmd)

	certcontrollerCmd.Flags().StringVar(&serviceName, "service-name", "mariadb-operator-webhook", "Webhook service name")
	certcontrollerCmd.Flags().StringVar(&serviceNamespace, "service-namespace", "default", "Webhook service namespace")
	certcontrollerCmd.Flags().StringVar(&secretName, "secret-name", "mariadb-operator-webhook", "Secret to store certs for webhook")
	certcontrollerCmd.Flags().StringVar(&secretNamespace, "secret-namespace", "default", "namespace of the secret to store certs")
	certcontrollerCmd.Flags().DurationVar(&webhookConfigRequeueInterval, "webhook-config-requeue-interval", time.Minute*5,
		"Time duration between reconciling webhook config for new certs")
}
