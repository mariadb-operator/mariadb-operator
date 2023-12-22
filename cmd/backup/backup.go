package backup

import (
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog              = ctrl.Log.WithName("setup")
	backupPath            string
	backupTargetPath      string
	targetRecoveryTimeRaw string
)

func init() {
	RootCmd.PersistentFlags().StringVar(&backupPath, "backup-path", "/backup", "Directory path where the backups are located.")
	RootCmd.PersistentFlags().StringVar(&backupTargetPath, "backup-target-path", "/backup/0-backup-target.txt",
		"Path to a file that contains the name of the backup target file.")
	RootCmd.PersistentFlags().StringVar(&targetRecoveryTimeRaw, "target-recovery-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.")
}

var RootCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup.",
	Long:  `Run and operate MariaDB in a cloud native way.`,
	Args:  cobra.NoArgs,
	Run:   func(cmd *cobra.Command, args []string) {},
}

func setupLogger(cmd *cobra.Command) error {
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return fmt.Errorf("error getting 'log-level' flag: %v\n", err)
	}
	logTimeEncoder, err := cmd.Flags().GetString("log-time-encoder")
	if err != nil {
		return fmt.Errorf("error getting 'log-time-encoder' flag: %v\n", err)
	}
	logDev, err := cmd.Flags().GetBool("log-dev")
	if err != nil {
		return fmt.Errorf("error getting 'log-dev' flag: %v\n", err)
	}
	log.SetupLogger(logLevel, logTimeEncoder, logDev)
}
