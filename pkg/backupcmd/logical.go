package backupcmd

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type logicalBackup struct {
	*CommandOpts
}

func (l *logicalBackup) BackupCommand(backup *databasev1alpha1.BackupMariaDB) *Command {
	cmds := []string{
		"echo '💾 Taking backup'",
		fmt.Sprintf(
			"mysqldump -h %s -P %d --lock-tables --all-databases > %s",
			l.MariaDB.Name,
			l.MariaDB.Spec.Port,
			l.backupPath(),
		),
	}

	if l.Cleanup {
		cmds = append(cmds,
			"echo '🧹 Cleaning up old backups'",
			fmt.Sprintf(
				"find %s -name *.sql -type f -mtime +%d -exec rm {} ';'",
				l.BasePath,
				backup.Spec.MaxRetentionDays,
			),
		)
	}

	if l.History {
		cmds = append(cmds,
			"echo '📜 Backup history'",
			fmt.Sprintf(
				"find %s -name *.sql -type f -printf '%s' | sort",
				l.BasePath,
				"%f\n",
			),
		)
	}

	return execCommand(cmds)
}

func (l *logicalBackup) RestoreCommand() *Command {
	cmds := []string{
		fmt.Sprintf(
			"export RESTORE_BACKUP=%s/%s",
			l.BasePath,
			l.restorePath(),
		),
		"echo '💾 Restoring backup: '$RESTORE_BACKUP''",
		fmt.Sprintf(
			"mysql -h %s -P %d < $RESTORE_BACKUP",
			l.MariaDB.Name,
			l.MariaDB.Spec.Port,
		),
	}
	return execCommand(cmds)
}

func (l *logicalBackup) backupPath() string {
	if l.BackupFile != "" {
		return fmt.Sprintf("%s/%s", l.BasePath, l.BackupFile)
	}
	return fmt.Sprintf(
		"%s/backup.$(date -u +'%s').sql",
		l.BasePath,
		"%Y-%m-%dT%H:%M:%SZ",
	)
}

func (l *logicalBackup) restorePath() string {
	if l.BackupFile != "" {
		return fmt.Sprintf("%s/%s", l.BasePath, l.BackupFile)
	}
	return fmt.Sprintf(
		"%s/$(find %s -name *.sql -type f -printf '%s' | sort | tail -n 1)",
		l.BasePath,
		l.BackupFile,
		"%f\n",
	)
}
