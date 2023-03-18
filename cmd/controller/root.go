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

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/controllers"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme                   = runtime.NewScheme()
	setupLog                 = ctrl.Log.WithName("setup")
	metricsAddr              string
	healthAddr               string
	leaderElect              bool
	logLevel                 string
	logTimeEncoder           string
	logDev                   bool
	serviceMonitorReconciler bool
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
}

var rootCmd = &cobra.Command{
	Use:   "mariadb-operator",
	Short: "MariaDB operator.",
	Long:  `This operator reconciles MariaDB resources so you can declaratively manage your instance using Kubernetes CRDs.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		setupLogger()

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:                 scheme,
			MetricsBindAddress:     metricsAddr,
			Port:                   9443,
			HealthProbeBindAddress: healthAddr,
			LeaderElection:         leaderElect,
			LeaderElectionID:       "mariadb-operator.mmontes.io",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		builder := builder.New(mgr.GetScheme())
		refResolver := refresolver.New(mgr.GetClient())
		conditionReady := conditions.NewReady()
		conditionComplete := conditions.NewComplete(mgr.GetClient())
		batchReconciler := batch.NewBatchReconciler(mgr.GetClient(), refResolver, builder)

		if err = (&controllers.MariaDBReconciler{
			Client:                   mgr.GetClient(),
			Scheme:                   mgr.GetScheme(),
			Builder:                  builder,
			RefResolver:              refResolver,
			ConditionReady:           conditionReady,
			ConfigMapReconciler:      configmap.NewConfigMapReconciler(mgr.GetClient(), builder, "my.cnf"),
			ServiceMonitorReconciler: serviceMonitorReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "MariaDB")
			os.Exit(1)
		}
		if err = (&controllers.BackupReconciler{
			Client:            mgr.GetClient(),
			Scheme:            mgr.GetScheme(),
			Builder:           builder,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			BatchReconciler:   batchReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Backup")
			os.Exit(1)
		}
		if err = (&controllers.RestoreReconciler{
			Client:            mgr.GetClient(),
			Scheme:            mgr.GetScheme(),
			Builder:           builder,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			BatchReconciler:   batchReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "restore")
			os.Exit(1)
		}
		if err = (&controllers.UserReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "User")
			os.Exit(1)
		}
		if err = (&controllers.GrantReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Grant")
			os.Exit(1)
		}
		if err = (&controllers.DatabaseReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Database")
			os.Exit(1)
		}
		if err = (&controllers.ConnectionReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Builder:        builder,
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Connection")
			os.Exit(1)
		}
		if err = (&controllers.SqlJobReconciler{
			Client:              mgr.GetClient(),
			Scheme:              mgr.GetScheme(),
			Builder:             builder,
			RefResolver:         refResolver,
			ConfigMapReconciler: configmap.NewConfigMapReconciler(mgr.GetClient(), builder, "job.sql"),
			ConditionComplete:   conditionComplete,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "SqlJob")
			os.Exit(1)
		}

		setupLog.Info("starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level to use, one of: "+
		"debug, info, warn, error, dpanic, panic, fatal.")
	rootCmd.PersistentFlags().StringVar(&logTimeEncoder, "log-time-encoder", "epoch", "Log time encoder to use, one of: "+
		"epoch, millis, nano, iso8601, rfc3339 or rfc3339nano")
	rootCmd.PersistentFlags().BoolVar(&logDev, "log-dev", false, "Enable development logs.")

	rootCmd.Flags().BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for controller manager.")
	rootCmd.Flags().BoolVar(&serviceMonitorReconciler, "service-monitor-reconciler", false, "Enable ServiceMonitor reconciler. "+
		"Enabling this requires Prometheus CRDs installed in the cluster.")
}

func setupLogger() {
	var lvl zapcore.Level
	var enc zapcore.TimeEncoder

	lvlErr := lvl.UnmarshalText([]byte(logLevel))
	if lvlErr != nil {
		setupLog.Error(lvlErr, "error unmarshalling log level")
		os.Exit(1)
	}
	encErr := enc.UnmarshalText([]byte(logTimeEncoder))
	if encErr != nil {
		setupLog.Error(encErr, "error unmarshalling time encoder")
		os.Exit(1)
	}
	opts := zap.Options{
		Level:       lvl,
		TimeEncoder: enc,
		Development: logDev,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
}
