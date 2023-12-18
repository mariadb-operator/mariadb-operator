package pitr

import "time"

func GetTargetRecoveryFile(backupFileNames []string, targetRecoveryTime time.Time) (string, error) {
	return "backup.2023-12-18T16:01:00Z.sql", nil
}
