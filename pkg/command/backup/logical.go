package backup

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
)

type logicalBackup struct {
	*BackupOpts
}

func (l *logicalBackup) BackupCommand(backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) *command.Command {
	cmds := []string{
		"set -euo pipefail",
		"echo '💾 Taking logical backup'",
		fmt.Sprintf(
			"mysqldump %s --lock-tables --all-databases > %s",
			command.ConnectionFlags(&l.BackupOpts.CommandOpts, mariadb),
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
	return command.ExecCommand(cmds)
}

func (l *logicalBackup) RestoreCommand(mariadb *mariadbv1alpha1.MariaDB) *command.Command {
	restorePath := l.restorePath()
	cmds := []string{
		fmt.Sprintf(
			"echo '💾 Restoring logical backup: '%s''",
			restorePath,
		),
		fmt.Sprintf(
			"mysql %s < %s",
			command.ConnectionFlags(&l.BackupOpts.CommandOpts, mariadb),
			restorePath,
		),
	}
	return command.ExecCommand(cmds)
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
