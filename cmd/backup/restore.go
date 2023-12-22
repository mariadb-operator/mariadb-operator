package backup

import (
	"os"
	"time"

	"github.com/mariadb-operator/mariadb-operator/pkg/pitr"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var PitrCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore.",
	Long:  `Finds the backup file to restore taking into account the target recovery time.`,
	Run: func(cmd *cobra.Command, args []string) {

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

		if err := os.WriteFile(backupTargetPath, []byte(targetRecoveryFile), 0644); err != nil {
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
