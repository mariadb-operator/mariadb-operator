package backupcmd

import (
	"fmt"

	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type physicalBackup struct {
	*CommandOpts
}

func (l *physicalBackup) BackupCommand(backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) *Command {
	cmds := []string{
		"echo '💾 Taking physical backup'",
		fmt.Sprintf(
			"mariabackup %s --backup --target-dir=%s",
			authFlags(l.CommandOpts, mariadb),
			l.backupPath(),
		),
		"echo '🧹 Cleaning up old backups'",
		fmt.Sprintf(
			"find %s -name *.sql -type d -mtime +%d -exec rm -rf {} ';'",
			l.BasePath,
			backup.Spec.MaxRetentionDays,
		),
		"echo '📜 Backup history'",
		fmt.Sprintf(
			"find %s -name *.sql -type d -printf '%s' | sort",
			l.BasePath,
			"%f\n",
		),
	}
	return execCommand(cmds)
}

func (l *physicalBackup) RestoreCommand(mariadb *mariadbv1alpha1.MariaDB) *Command {
	restorePath := l.restorePath()
	cmds := []string{
		fmt.Sprintf(
			"echo '💾 Restoring physical backup: '%s''",
			restorePath,
		),
		fmt.Sprintf(
			"mariabackup %s --prepare --target-dir=%s",
			authFlags(l.CommandOpts, mariadb),
			restorePath,
		),
	}
	return execCommand(cmds)
}

func (l *physicalBackup) backupPath() string {
	if l.BackupFile != "" {
		return fmt.Sprintf("%s/%s", l.BasePath, l.BackupFile)
	}
	return fmt.Sprintf(
		"%s/backup.$(date -u +'%s').sql",
		l.BasePath,
		"%Y-%m-%dT%H:%M:%SZ",
	)
}

func (l *physicalBackup) restorePath() string {
	if l.BackupFile != "" {
		return fmt.Sprintf("%s/%s", l.BasePath, l.BackupFile)
	}
	return fmt.Sprintf(
		"%s/$(find %s -name *.sql -type d -printf '%s' | sort | tail -n 1)",
		l.BasePath,
		l.BasePath,
		"%f\n",
	)
}
