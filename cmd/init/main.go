package init

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/config"
	"github.com/mariadb-operator/mariadb-operator/pkg/galera/filemanager"
	kubeclientset "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/clientset"
	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"github.com/sethvargo/go-envconfig"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger           = ctrl.Log
	configDir        string
	stateDir         string
	mariadbName      string
	mariadbNamespace string
)

func init() {
	RootCmd.Flags().StringVar(&configDir, "config-dir", "/etc/mysql/mariadb.conf.d",
		"The directory that contains MariaDB configuration files")
	RootCmd.Flags().StringVar(&stateDir, "state-dir", "/var/lib/mysql", "The directory that contains MariaDB state files")
	RootCmd.Flags().StringVar(&mariadbName, "mariadb-name", "", "The name of the MariaDB to be initialized")
	RootCmd.Flags().StringVar(&mariadbNamespace, "mariadb-namespace", "", "The namespace of the MariaDB to be initialized")
}

var RootCmd = &cobra.Command{
	Use:   "init",
	Short: "Init.",
	Long:  `Init container for Galera and co-operates with mariadb-operator.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting init")

		ctx, cancel := newContext()
		defer cancel()

		env, err := getEnv(ctx)
		if err != nil {
			logger.Error(err, "Error getting environment variables")
			os.Exit(1)
		}

		clientSet, err := kubeclientset.NewClientSet()
		if err != nil {
			logger.Error(err, "Error creating Kubernetes clientset")
			os.Exit(1)
		}
		mdb, err := clientSet.GetMariaDB(ctx, mariadbName, mariadbNamespace)
		if err != nil {
			logger.Error(err, "Error getting MariaDB")
			os.Exit(1)
		}

		fileManager, err := filemanager.NewFileManager(configDir, stateDir)
		if err != nil {
			logger.Error(err, "Error creating file manager")
			os.Exit(1)
		}
		configBytes, err := config.NewConfigFile(mdb).Marshal(env.PodName, env.MariadbRootPassword)
		if err != nil {
			logger.Error(err, "Error getting Galera config")
			os.Exit(1)
		}
		logger.Info("Configuring Galera")
		if err := fileManager.WriteConfigFile(config.ConfigFileName, configBytes); err != nil {
			logger.Error(err, "Error writing Galera config")
			os.Exit(1)
		}

		entries, err := os.ReadDir(stateDir)
		if err != nil {
			logger.Error(err, "Error reading state directory")
			os.Exit(1)
		}
		if len(entries) > 0 {
			info, err := os.Stat(path.Join(stateDir, "grastate.dat"))
			if !os.IsNotExist(err) && info.Size() > 0 {
				logger.Info("Already initialized. Init done")
				os.Exit(0)
			}
		}

		idx, err := statefulset.PodIndex(env.PodName)
		if err != nil {
			logger.Error(err, "error getting index from Pod", "pod", env.PodName)
			os.Exit(1)
		}
		if *idx == 0 {
			logger.Info("Configuring bootstrap")
			if err := fileManager.WriteConfigFile(config.BootstrapFileName, config.BootstrapFile); err != nil {
				logger.Error(err, "Error writing bootstrap config")
				os.Exit(1)
			}
			logger.Info("Init done")
			os.Exit(0)
		}

		previousPodName, err := previousPodName(mdb, *idx)
		if err != nil {
			logger.Error(err, "Error getting previous Pod")
			os.Exit(1)
		}
		logger.Info("Waiting for previous Pod to be ready", "pod", previousPodName)
		if err := waitForPodReady(ctx, mdb, previousPodName, clientSet, logger); err != nil {
			logger.Error(err, "Error waiting for previous Pod to be ready", "pod", previousPodName)
			os.Exit(1)
		}
		logger.Info("Init done")
	},
}

type environment struct {
	PodName             string `env:"POD_NAME,required"`
	MariadbRootPassword string `env:"MARIADB_ROOT_PASSWORD,required"`
}

func getEnv(ctx context.Context) (*environment, error) {
	var env environment
	if err := envconfig.Process(ctx, &env); err != nil {
		return nil, err
	}
	return &env, nil
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

func previousPodName(mariadb *mariadbv1alpha1.MariaDB, podIndex int) (string, error) {
	if podIndex == 0 {
		return "", fmt.Errorf("Pod '%s' is the first Pod", statefulset.PodName(mariadb.ObjectMeta, podIndex))
	}
	previousPodIndex := podIndex - 1
	return statefulset.PodName(mariadb.ObjectMeta, previousPodIndex), nil
}

func waitForPodReady(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, name string, clientset *kubeclientset.ClientSet,
	logger logr.Logger) error {
	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(mariadb.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			logger.V(1).Info("Error getting Pod", "err", err)
			return false, nil
		}
		if !mariadbpod.PodReady(pod) {
			logger.V(1).Info("Pod not ready", "pod", previousPodName)
			return false, nil
		}
		logger.V(1).Info("Pod ready", "pod", previousPodName)
		return true, nil
	})
}
