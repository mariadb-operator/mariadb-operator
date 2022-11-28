package backupcmd

import (
	"fmt"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type logicalBackup struct {
	*CommandOpts
}

func (l *logicalBackup) BackupCommand(backup *databasev1alpha1.BackupMariaDB,
	mariadb *databasev1alpha1.MariaDB) *Command {
	cmds := []string{
		"echo '💾 Taking logical backup'",
		fmt.Sprintf(
			"mysqldump %s --lock-tables --all-databases > %s",
			authFlags(l.CommandOpts, mariadb),
			l.backupPath(),
		),
		"echo '🧹 Cleaning up old backups'",
		fmt.Sprintf(
			"find %s -name *.sql -type f -mtime +%d -exec rm {} ';'",
			l.BasePath,
			backup.Spec.MaxRetentionDays,
		),
		"echo '📜 Backup history'",
		fmt.Sprintf(
			"find %s -name *.sql -type f -printf '%s' | sort",
			l.BasePath,
			"%f\n",
		),
	}
	return execCommand(cmds)
}

func (l *logicalBackup) RestoreCommand(mariadb *databasev1alpha1.MariaDB) *Command {
	restorePath := l.restorePath()
	cmds := []string{
		fmt.Sprintf(
			"echo '💾 Restoring physical backup: '%s''",
			restorePath,
		),
		fmt.Sprintf(
			"mysql %s < %s",
			authFlags(l.CommandOpts, mariadb),
			restorePath,
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
		l.BasePath,
		"%f\n",
	)
}
