package pitr

import (
	"fmt"
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/pkg/pitr"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog              = ctrl.Log.WithName("setup")
	backupPath            string
	resultFilePath        string
	targetRecoveryTimeRaw string
)

func init() {
	PitrCmd.Flags().StringVar(&backupPath, "backup-path", "/backup", "Directory path where the backups are located.")
	PitrCmd.Flags().StringVar(&resultFilePath, "result-file-path", "/backup/0-point-in-time-recovery.txt",
		"Result file containing the backup file to restore.")
	PitrCmd.Flags().StringVar(&targetRecoveryTimeRaw, "target-recovery-time", "",
		"RFC3339 (1970-01-01T00:00:00Z) date and time that defines the point in time recovery objective.")
}

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

		setupLog.Info("Starting PITR")
		targetRecoveryTime, err := time.Parse(time.RFC3339, targetRecoveryTimeRaw)
		if err != nil {
			setupLog.Error(err, "error parsing target recovery time")
			os.Exit(1)
		}
		setupLog.Info("Target recovery time", "time", targetRecoveryTime.String())

		backupFileNames, err := getBackupFileNames()
		if err != nil {
			setupLog.Error(err, "error reading backup files", "path", backupPath)
			os.Exit(1)
		}

		targetRecoveryFile, err := pitr.GetTargetRecoveryFile(backupFileNames, targetRecoveryTime, ctrl.Log.WithName("pitr"))
		if err != nil {
			setupLog.Error(err, "error reading getting target recovery file", "time", targetRecoveryTime)
			os.Exit(1)
		}

		if err := os.WriteFile(resultFilePath, []byte(targetRecoveryFile), 0644); err != nil {
			setupLog.Error(err, "error writing target recovery file")
			os.Exit(1)
		}
	},
}

func getBackupFileNames() ([]string, error) {
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, e := range entries {
		name := e.Name()
		if pitr.IsValidBackupFile(name) {
			fileNames = append(fileNames, name)
		} else {
			setupLog.V(1).Info("ignoring file", "file", name)
		}
	}
	return fileNames, nil
}
