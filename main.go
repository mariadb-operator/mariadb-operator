/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/controllers"
	"github.com/mmontes11/mariadb-operator/pkg/builder"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/controller/batch"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(databasev1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var leaderElection bool
	var leaderElectionId string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&leaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionId, "leader-elect-id", "e5c434f5.mmontes.io", "Leader election ID for controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElection,
		LeaderElectionID:       leaderElectionId,
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
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Builder:        builder,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MariaDB")
		os.Exit(1)
	}
	if err = (&controllers.BackupMariaDBReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Builder:           builder,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		BatchReconciler:   batchReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupMariaDB")
		os.Exit(1)
	}
	if err = (&controllers.RestoreMariaDBReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Builder:           builder,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		BatchReconciler:   batchReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RestoreMariaDB")
		os.Exit(1)
	}
	if err = (&controllers.UserMariaDBReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "UserMariaDB")
		os.Exit(1)
	}
	if err = (&controllers.GrantMariaDBReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GrantMariaDB")
		os.Exit(1)
	}
	if err = (&controllers.DatabaseMariaDBReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DatabaseMariaDB")
		os.Exit(1)
	}

	if err = (&databasev1alpha1.MariaDB{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "MariaDB")
		os.Exit(1)
	}
	if err = (&databasev1alpha1.BackupMariaDB{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "BackupMariaDB")
		os.Exit(1)
	}
	if err = (&databasev1alpha1.RestoreMariaDB{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "RestoreMariaDB")
		os.Exit(1)
	}
	if err = (&databasev1alpha1.UserMariaDB{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "UserMariaDB")
		os.Exit(1)
	}
	if err = (&databasev1alpha1.GrantMariaDB{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "GrantMariaDB")
		os.Exit(1)
	}
	if err = (&databasev1alpha1.DatabaseMariaDB{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "DatabaseMariaDB")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
