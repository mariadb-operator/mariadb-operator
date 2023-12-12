package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/controller"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/batch"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/endpoints"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/galera"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme                   = runtime.NewScheme()
	setupLog                 = ctrl.Log.WithName("setup")
	metricsAddr              string
	healthAddr               string
	logLevel                 string
	logTimeEncoder           string
	logDev                   bool
	leaderElect              bool
	serviceMonitorReconciler bool
	requeueConnection        time.Duration
	requeueSqlJob            time.Duration
	webhookPort              int
	webhookCertDir           string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	rootCmd.PersistentFlags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	rootCmd.PersistentFlags().StringVar(&healthAddr, "health-addr", ":8081", "The address the probe endpoint binds to.")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level to use, one of: "+
		"debug, info, warn, error, dpanic, panic, fatal.")
	rootCmd.PersistentFlags().StringVar(&logTimeEncoder, "log-time-encoder", "epoch", "Log time encoder to use, one of: "+
		"epoch, millis, nano, iso8601, rfc3339 or rfc3339nano")
	rootCmd.PersistentFlags().BoolVar(&logDev, "log-dev", false, "Enable development logs.")
	rootCmd.PersistentFlags().BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for controller manager.")
	rootCmd.Flags().BoolVar(&serviceMonitorReconciler, "service-monitor-reconciler", false, "Enable ServiceMonitor reconciler. "+
		"Enabling this requires Prometheus CRDs installed in the cluster.")
	rootCmd.Flags().DurationVar(&requeueConnection, "requeue-connection", 10*time.Second, "The interval at which Connections are requeued.")
	rootCmd.Flags().DurationVar(&requeueSqlJob, "requeue-sqljob", 10*time.Second, "The interval at which SqlJobs are requeued.")
	rootCmd.Flags().IntVar(&webhookPort, "webhook-port", 9443, "Port to be used by the webhook server.")
	rootCmd.Flags().StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"Directory containing the TLS certificate for the webhook server. 'tls.crt' and 'tls.key' must be present in this directory.")
}

var rootCmd = &cobra.Command{
	Use:   "mariadb-operator-enterprise",
	Short: "MariaDB Operator Enterprise.",
	Long:  `Run and operate MariaDB Enterprise in OpenShift.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetupLogger(logLevel, logTimeEncoder, logDev)

		ctx, cancel := signal.NotifyContext(context.Background(), []os.Signal{
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGKILL,
			syscall.SIGHUP,
			syscall.SIGQUIT}...,
		)
		defer cancel()

		restConfig, err := ctrl.GetConfig()
		if err != nil {
			setupLog.Error(err, "Unable to get config")
			os.Exit(1)
		}

		mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: healthAddr,
			LeaderElection:         leaderElect,
			LeaderElectionID:       "mariadb-operator-enterprisse.k8s.mariadb.com",
			WebhookServer: webhook.NewServer(webhook.Options{
				CertDir: webhookCertDir,
				Port:    webhookPort,
			}),
		})
		if err != nil {
			setupLog.Error(err, "Unable to start manager")
			os.Exit(1)
		}

		client := mgr.GetClient()
		scheme := mgr.GetScheme()
		galeraRecorder := mgr.GetEventRecorderFor("galera")
		replRecorder := mgr.GetEventRecorderFor("replication")

		env, err := environment.GetEnvironment(ctx)
		if err != nil {
			setupLog.Error(err, "Error getting environment")
			os.Exit(1)
		}
		discoveryClient, err := discovery.NewDiscoveryClient(restConfig)
		if err != nil {
			setupLog.Error(err, "Error getting discovery client")
			os.Exit(1)
		}

		builder := builder.NewBuilder(scheme, env)
		refResolver := refresolver.New(client)

		conditionReady := condition.NewReady()
		conditionComplete := condition.NewComplete(client)

		configMapReconciler := configmap.NewConfigMapReconciler(client, builder)
		secretReconciler := secret.NewSecretReconciler(client, builder)
		serviceReconciler := service.NewServiceReconciler(client)
		endpointsReconciler := endpoints.NewEndpointsReconciler(client, builder)
		batchReconciler := batch.NewBatchReconciler(client, builder)
		rbacReconciler := rbac.NewRBACReconiler(client, builder)

		replConfig := replication.NewReplicationConfig(client, builder, secretReconciler)
		replicationReconciler := replication.NewReplicationReconciler(
			client,
			replRecorder,
			builder,
			replConfig,
			replication.WithRefResolver(refResolver),
			replication.WithSecretReconciler(secretReconciler),
			replication.WithServiceReconciler(serviceReconciler),
		)
		galeraReconciler := galera.NewGaleraReconciler(
			client,
			galeraRecorder,
			env,
			builder,
			galera.WithRefResolver(refResolver),
			galera.WithConfigMapReconciler(configMapReconciler),
			galera.WithServiceReconciler(serviceReconciler),
		)

		podReplicationController := controller.NewPodController(
			client,
			refResolver,
			controller.NewPodReplicationController(
				client,
				replRecorder,
				builder,
				refResolver,
				replConfig,
			),
			[]string{
				metadata.MariadbAnnotation,
				metadata.ReplicationAnnotation,
			},
		)
		podGaleraController := controller.NewPodController(
			client,
			refResolver,
			controller.NewPodGaleraController(client, galeraRecorder),
			[]string{
				metadata.MariadbAnnotation,
				metadata.GaleraAnnotation,
			},
		)

		if err = (&controller.MariaDBReconciler{
			Client: client,
			Scheme: scheme,

			Environment:     env,
			Builder:         builder,
			RefResolver:     refResolver,
			ConditionReady:  conditionReady,
			DiscoveryClient: discoveryClient,

			ConfigMapReconciler: configMapReconciler,
			SecretReconciler:    secretReconciler,
			ServiceReconciler:   serviceReconciler,
			EndpointsReconciler: endpointsReconciler,
			RBACReconciler:      rbacReconciler,

			ReplicationReconciler: replicationReconciler,
			GaleraReconciler:      galeraReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "MariaDB")
			os.Exit(1)
		}
		if err = (&controller.BackupReconciler{
			Client:            client,
			Scheme:            scheme,
			Builder:           builder,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			BatchReconciler:   batchReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Backup")
			os.Exit(1)
		}
		if err = (&controller.RestoreReconciler{
			Client:            client,
			Scheme:            scheme,
			Builder:           builder,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			BatchReconciler:   batchReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "restore")
			os.Exit(1)
		}
		if err = (&controller.UserReconciler{
			Client:         client,
			Scheme:         scheme,
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "User")
			os.Exit(1)
		}
		if err = (&controller.GrantReconciler{
			Client:         client,
			Scheme:         scheme,
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Grant")
			os.Exit(1)
		}
		if err = (&controller.DatabaseReconciler{
			Client:         client,
			Scheme:         scheme,
			RefResolver:    refResolver,
			ConditionReady: conditionReady,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Database")
			os.Exit(1)
		}
		if err = (&controller.ConnectionReconciler{
			Client:          client,
			Scheme:          scheme,
			Builder:         builder,
			RefResolver:     refResolver,
			ConditionReady:  conditionReady,
			RequeueInterval: requeueConnection,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Connection")
			os.Exit(1)
		}
		if err = (&controller.SqlJobReconciler{
			Client:              client,
			Scheme:              scheme,
			Builder:             builder,
			RefResolver:         refResolver,
			ConfigMapReconciler: configMapReconciler,
			ConditionComplete:   conditionComplete,
			RequeueInterval:     requeueSqlJob,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "SqlJob")
			os.Exit(1)
		}
		if err = podReplicationController.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "PodReplication")
			os.Exit(1)
		}
		if err := podGaleraController.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "PodGalera")
			os.Exit(1)
		}
		if err = (&controller.StatefulSetGaleraReconciler{
			Client:      client,
			RefResolver: refResolver,
			Recorder:    galeraRecorder,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "StatefulSetGalera")
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

		if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			setupLog.Error(err, "Unable to set up health check")
			os.Exit(1)
		}
		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			setupLog.Error(err, "Unable to set up ready check")
			os.Exit(1)
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Error running manager")
			os.Exit(1)
		}
	},
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
