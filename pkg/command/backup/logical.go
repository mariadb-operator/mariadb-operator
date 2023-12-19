package backup

import (
	"errors"
	"fmt"
	"strings"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/pkg/pitr"
)

type logicalBackup struct {
	*BackupOpts
}

func (l *logicalBackup) BackupCommand(backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) *command.Command {
	dumpOpts := "--single-transaction --events --routines --dump-slave=2 --master-data=2 --gtid --all-databases"
	if l.BackupOpts.DumpOpts != nil {
		dumpOpts = strings.Join(l.BackupOpts.DumpOpts, " ")
	}
	cmds := []string{
		"set -euo pipefail",
		"echo '💾 Taking logical backup'",
		fmt.Sprintf(
			"mariadb-dump %s %s > %s",
			command.ConnectionFlags(&l.BackupOpts.CommandOpts, mariadb),
			dumpOpts,
			l.backupPath(),
		),
		"echo '🧹 Cleaning up old backups'",
		fmt.Sprintf(
			"find %s -name *.sql -type f -mtime +%d -delete",
			l.BackupPath,
			backup.Spec.MaxRetentionDays,
		),
		"echo '📜 Backup history'",
		fmt.Sprintf(
			"find %s -name *.sql -type f -printf '%s' | sort",
			l.BackupPath,
			"%f\n",
		),
	}
	return command.NewBashCommand(cmds)
}

func (l *logicalBackup) PitrCommand() (*command.Command, error) {
	if l.PitrFile == "" {
		return nil, errors.New("PitrFile must be set")
	}
	if l.PitrTime == nil {
		return nil, errors.New("PitrTime must be set")
	}
	args := []string{
		"pitr",
		"--backup-path",
		l.BackupPath,
		"--result-file-path",
		l.PitrFile,
		"--target-recovery-time",
		pitr.FormatBackupDate(*l.PitrTime),
	}
	return command.NewCommand(nil, args), nil
}

func (l *logicalBackup) RestoreCommand(mariadb *mariadbv1alpha1.MariaDB) *command.Command {
	restorePath := l.restorePath()
	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(
			"echo '💾 Restoring logical backup: '%s''",
			restorePath,
		),
		fmt.Sprintf(
			"mariadb %s < %s",
			command.ConnectionFlags(&l.BackupOpts.CommandOpts, mariadb),
			restorePath,
		),
	}
	return command.NewBashCommand(cmds)
}

func (l *logicalBackup) backupPath() string {
	if pitrPath := l.pitrPath(); pitrPath != "" {
		return pitrPath
	}
	return fmt.Sprintf(
		"%s/backup.$(date -u +'%s').sql",
		l.BackupPath,
		"%Y-%m-%dT%H:%M:%SZ",
	)
}

func (l *logicalBackup) restorePath() string {
	if pitrPath := l.pitrPath(); pitrPath != "" {
		return pitrPath
	}
	return fmt.Sprintf(
		"%s/$(find %s -name *.sql -type f -printf '%s' | sort | tail -n 1)",
		l.BackupPath,
		l.BackupPath,
		"%f\n",
	)
}

func (l *logicalBackup) pitrPath() string {
	if l.PitrFile != "" {
		return fmt.Sprintf("%s/$(cat '%s')", l.BackupPath, l.PitrFile)
	}
	return ""
}
