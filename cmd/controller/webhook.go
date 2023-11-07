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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	caDir   string
	certDir string
	dnsName string
	port    int

	tlsCert = "tls.crt"
	tlsKey  = "tls.key"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "MariaDB operator webhook server.",
	Long:  `Provides validation and inmutability checks for MariaDB resources.`,
	Run: func(cmd *cobra.Command, args []string) {
		setupLogger()

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: healthAddr,
			WebhookServer: webhook.NewServer(webhook.Options{
				CertDir: certDir,
				Port:    port,
			}),
		})
		if err != nil {
			setupLog.Error(err, "Unable to start manager")
			os.Exit(1)
		}

		if err = (&mariadbv1alpha1.MariaDB{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "MariaDB")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.Backup{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "Backup")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.Restore{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "restore")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.User{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "User")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.Grant{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "Grant")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.Database{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "Database")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.Connection{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "Connection")
			os.Exit(1)
		}
		if err = (&mariadbv1alpha1.SqlJob{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create webhook", "webhook", "SqlJob")
			os.Exit(1)
		}

		if err := mgr.AddReadyzCheck("certs", func(_ *http.Request) error {
			return checkCerts(dnsName, time.Now(), setupLog)
		}); err != nil {
			setupLog.Error(err, "Unable to add readyz check")
			os.Exit(1)
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Error running manager")
			os.Exit(1)
		}
	},
}

func checkCerts(dnsName string, at time.Time, logger logr.Logger) error {
	caKeyPair, err := readKeyPair(caDir)
	if err != nil {
		logger.Error(err, "Error reading CA KeyPair")
		return err
	}
	certKeyPair, err := readKeyPair(certDir)
	if err != nil {
		logger.Error(err, "Error reading certificate KeyPair")
		return err
	}
	valid, err := pki.ValidCert(caKeyPair, certKeyPair, dnsName, at)
	if !valid || err != nil {
		err := fmt.Errorf("Certificate is not valid for %s", dnsName)
		logger.Error(err, "Error validating certificate")
		return err
	}
	return nil
}

func readKeyPair(dir string) (*pki.KeyPair, error) {
	certFile := filepath.Join(dir, tlsCert)
	if _, err := os.Stat(certFile); err != nil {
		return nil, err
	}
	keyFile := filepath.Join(dir, tlsKey)
	if _, err := os.Stat(certFile); err != nil {
		return nil, err
	}
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return pki.KeyPairFromPEM(certBytes, keyBytes)
}

func init() {
	rootCmd.AddCommand(webhookCmd)
	webhookCmd.Flags().StringVar(&caDir, "ca-dir", "/tmp/k8s-webhook-server/certificate-authority",
		"Path containing the CA TLS certificate for the webhook server.")
	webhookCmd.Flags().StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"Path containing the TLS certificate for the webhook server.")
	webhookCmd.Flags().StringVar(&dnsName, "dns-name", "mariadb-operator-webhook.default.svc",
		"TLS certificate DNS name.")
	webhookCmd.Flags().IntVar(&port, "port", 10250, "Port to be used by the webhook server.")
}
