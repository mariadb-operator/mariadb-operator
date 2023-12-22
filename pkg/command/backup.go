package command

import (
	"errors"
	"fmt"
	"strings"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/backup"
)

type BackupOpts struct {
	CommandOpts
	Path           string
	TargetFilePath string
	TargetTime     time.Time
	DumpOpts       []string
}

type BackupOpt func(*BackupOpts)

func WithBackup(path string, targetFilePath string, targetTime time.Time) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Path = path
		bo.TargetFilePath = targetFilePath
		bo.TargetTime = targetTime
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
		"echo '💾 Taking backup'",
		fmt.Sprintf(
			"mariadb-dump %s %s > %s",
			ConnectionFlags(&b.BackupOpts.CommandOpts, mariadb),
			dumpOpts,
			b.backupPath(),
		),
	}
	return NewBashCommand(cmds)
}

// TODO
func (b *BackupCommand) MariadbOperatorBackup() *Command {
	return nil
}

func (b *BackupCommand) MariadbOperatorRestore() *Command {
	args := []string{
		"backup",
		"restore",
		"--path",
		b.Path,
		"--target-time",
		backup.FormatBackupDate(b.TargetTime),
		"--target-file-path",
		b.TargetFilePath,
	}
	return NewCommand(nil, args)
}

func (b *BackupCommand) MariadbRestore(mariadb *mariadbv1alpha1.MariaDB) *Command {
	path := b.evalTargetFilePath()
	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(
			"echo '💾 Restoring backup: '%s''",
			path,
		),
		fmt.Sprintf(
			"mariadb %s < %s",
			ConnectionFlags(&b.BackupOpts.CommandOpts, mariadb),
			path,
		),
	}
	return NewBashCommand(cmds)
}

func (b *BackupCommand) backupPath() string {
	return fmt.Sprintf(
		"%s/backup.$(date -u +'%s').sql",
		b.Path,
		"%Y-%m-%dT%H:%M:%SZ",
	)
}
func (b *BackupCommand) evalTargetFilePath() string {
	return fmt.Sprintf("$(cat '%s')", b.TargetFilePath)
}
