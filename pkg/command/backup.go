package command

import (
	"errors"
	"fmt"
	"strings"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	backuppkg "github.com/mariadb-operator/mariadb-operator/pkg/backup"
)

type BackupOpts struct {
	CommandOpts
	Path                 string
	TargetFilePath       string
	MaxRetentionDuration time.Duration
	TargetTime           time.Time
	LogLevel             string
	DumpOpts             []string
}

type BackupOpt func(*BackupOpts)

func WithBackup(path string, targetFilePath string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Path = path
		bo.TargetFilePath = targetFilePath
	}
}

func WithBackupMaxRetentionDuration(d time.Duration) BackupOpt {
	return func(bo *BackupOpts) {
		bo.MaxRetentionDuration = d
	}
}

func WithBackupTargetTime(t time.Time) BackupOpt {
	return func(bo *BackupOpts) {
		bo.TargetTime = t
	}
}

func WithBackupDumpOpts(opts []string) BackupOpt {
	return func(o *BackupOpts) {
		o.DumpOpts = opts
	}
}

func WithBackupUserEnv(u string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.UserEnv = u
	}
}

func WithBackupPasswordEnv(p string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.PasswordEnv = p
	}
}

func WithBackupDatabase(d string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Database = &d
	}
}

func WithBackupLogLevel(l string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.LogLevel = l
	}
}

type BackupCommand struct {
	*BackupOpts
}

func NewBackupCommand(userOpts ...BackupOpt) (*BackupCommand, error) {
	opts := &BackupOpts{}
	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.Path == "" {
		return nil, errors.New("path not provided")
	}
	if opts.TargetFilePath == "" {
		return nil, errors.New("target file not provided")
	}
	if opts.MaxRetentionDuration == 0 {
		opts.MaxRetentionDuration = 30 * 24 * time.Hour
	}
	if opts.TargetTime == (time.Time{}) {
		opts.TargetTime = time.Now()
	}
	if opts.UserEnv == "" {
		return nil, errors.New("user environment variable not provided")
	}
	if opts.PasswordEnv == "" {
		return nil, errors.New("password environment variable not provided")
	}
	return &BackupCommand{opts}, nil
}

func (b *BackupCommand) MariadbDump(backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) *Command {
	dumpOpts := "--single-transaction --events --routines --dump-slave=2 --master-data=2 --gtid --all-databases"
	if b.BackupOpts.DumpOpts != nil {
		dumpOpts = strings.Join(b.BackupOpts.DumpOpts, " ")
	}
	cmds := []string{
		"set -euo pipefail",
		"echo 💾 Exporting env",
		fmt.Sprintf(
			"export BACKUP_FILE=%s",
			b.newBackupFilePath(),
		),
		fmt.Sprintf(
			"echo 💾 Writing target file: %s",
			b.TargetFilePath,
		),
		fmt.Sprintf(
			"echo \"${BACKUP_FILE}\" > %s",
			b.TargetFilePath,
		),
		"echo 💾 Setting target file permissions",
		fmt.Sprintf(
			"chmod 777 %s",
			b.TargetFilePath,
		),
		fmt.Sprintf(
			"echo 💾 Taking backup: %s",
			b.evalTargetFilePath(),
		),
		fmt.Sprintf(
			"mariadb-dump %s %s > %s",
			ConnectionFlags(&b.BackupOpts.CommandOpts, mariadb),
			dumpOpts,
			b.evalTargetFilePath(),
		),
	}
	return NewBashCommand(cmds)
}

func (b *BackupCommand) MariadbOperatorBackup() *Command {
	args := []string{
		"backup",
		"--path",
		b.Path,
		"--target-file-path",
		b.TargetFilePath,
		"--max-retention",
		b.MaxRetentionDuration.String(),
		"--log-level",
		b.LogLevel,
	}
	return NewCommand(nil, args)
}

func (b *BackupCommand) MariadbOperatorRestore() *Command {
	args := []string{
		"backup",
		"restore",
		"--path",
		b.Path,
		"--target-time",
		backuppkg.FormatBackupDate(b.TargetTime),
		"--target-file-path",
		b.TargetFilePath,
		"--log-level",
		b.LogLevel,
	}
	return NewCommand(nil, args)
}

func (b *BackupCommand) MariadbRestore(mariadb *mariadbv1alpha1.MariaDB) *Command {
	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(
			"echo 💾 Restoring backup: %s",
			b.evalTargetFilePath(),
		),
		fmt.Sprintf(
			"mariadb %s < %s",
			ConnectionFlags(&b.BackupOpts.CommandOpts, mariadb),
			b.evalTargetFilePath(),
		),
	}
	return NewBashCommand(cmds)
}

func (b *BackupCommand) newBackupFilePath() string {
	return fmt.Sprintf(
		"%s/backup.$(date -u +'%s').sql",
		b.Path,
		"%Y-%m-%dT%H:%M:%SZ",
	)
}

func (b *BackupCommand) evalTargetFilePath() string {
	return fmt.Sprintf("$(cat '%s')", b.TargetFilePath)
}
