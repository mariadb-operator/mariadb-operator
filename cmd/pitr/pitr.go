package pitr

import (
	"fmt"
	"os"

	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var setupLog = ctrl.Log.WithName("setup")

var PitrCmd = &cobra.Command{
	Use:   "pitr",
	Short: "PITR.",
	Long:  `Point In Time Recovery.`,
	Run: func(cmd *cobra.Command, args []string) {
		logLevel, err := cmd.Flags().GetString("log-level")
		if err != nil {
			fmt.Printf("error getting 'log-level' flag: %v\n", err)
			os.Exit(1)
		}
		logTimeEncoder, err := cmd.Flags().GetString("log-time-encoder")
		if err != nil {
			fmt.Printf("error getting 'log-time-encoder' flag: %v\n", err)
			os.Exit(1)
		}
		logDev, err := cmd.Flags().GetBool("log-dev")
		if err != nil {
			fmt.Printf("error getting 'log-dev' flag: %v\n", err)
			os.Exit(1)
		}
		log.SetupLogger(logLevel, logTimeEncoder, logDev)

		setupLog.Info("Starting PITR...")
	},
}
