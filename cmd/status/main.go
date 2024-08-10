package status

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/state"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme   = runtime.NewScheme()
	logger   = ctrl.Log
	stateDir string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))

	RootCmd.Flags().StringVar(&stateDir, "state-dir", "/var/lib/mysql", "The directory that contains MariaDB state files")
}

var RootCmd = &cobra.Command{
	Use:   "status",
	Short: "Status.",
	Long:  `Status container for Galera and co-operates with mariadb-operator.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Starting status")

		ctx, cancel := newContext()
		defer cancel()

		env, err := environment.GetPodEnv(ctx)
		if err != nil {
			logger.Error(err, "Error getting environment variables")
			os.Exit(1)
		}
		k8sClient, err := getK8sClient()
		if err != nil {
			logger.Error(err, "Error getting Kubernetes client")
			os.Exit(1)
		}
		state := state.NewState(stateDir)

		hasGaleraState, err := state.HasGaleraState()
		if err != nil {
			logger.Error(err, "Error checking Galera init state")
			os.Exit(1)
		}
		if !hasGaleraState {
			logger.Info("MariaDB not initialized. Skipping status patch...")
			os.Exit(0)
		}

		key := types.NamespacedName{
			Name:      env.MariadbName,
			Namespace: env.PodNamespace,
		}
		var mdb mariadbv1alpha1.MariaDB
		if err := k8sClient.Get(ctx, key, &mdb); err != nil {
			logger.Error(err, "Error getting MariaDB")
			os.Exit(1)
		}

		err = patchStatus(ctx, k8sClient, &mdb, func(status *mariadbv1alpha1.MariaDBStatus) {
			condition.SetGaleraConfigured(status)
			status.GaleraRecovery = nil
			condition.SetGaleraNotReady(status)
		})
		if err != nil {
			logger.Error(err, "Error patching MariaDB status")
			os.Exit(1)
		}
		logger.Info("MariaDB status successfully patched")
	},
}

func newContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT}...,
	)
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

func patchStatus(ctx context.Context, client client.Client, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := ctrlclient.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return client.Status().Patch(ctx, mariadb, patch)
}
