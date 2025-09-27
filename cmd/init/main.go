package init

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	scheme    = runtime.NewScheme()
	logger    = ctrl.Log
	configDir string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))

	RootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "/etc/mysql/mariadb.conf.d",
		"The directory that contains MariaDB configuration files")
	RootCmd.PersistentFlags().StringVar(&stateDir, "state-dir", "/var/lib/mysql", "The directory that contains MariaDB state files")

	RootCmd.AddCommand(galeraCommand)
	RootCmd.AddCommand(replicationCommand)
}

var RootCmd = &cobra.Command{
	Use:   "init",
	Short: "Init.",
	Long:  "Init container that co-operates with mariadb-operator.",
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
