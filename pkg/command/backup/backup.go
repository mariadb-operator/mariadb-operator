package backup

import (
	"errors"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
)

type Commander interface {
	BackupCommand(backup *mariadbv1alpha1.Backup, mariadb *mariadbv1alpha1.MariaDB) *command.Command
	PitrCommand() (*command.Command, error)
	RestoreCommand(mariadb *mariadbv1alpha1.MariaDB) *command.Command
}

type BackupOpts struct {
	command.CommandOpts
	BackupPath         string
	TargetRecoveryFile string
	PitrFile           string
	PitrTime           *time.Time
	DumpOpts           []string
}

type Option func(*BackupOpts)

func WithBackupPath(backupPath string) Option {
	return func(o *BackupOpts) {
		o.BackupPath = backupPath
	}
}

func WithTargetRecoveryFile(targetRecoveryFile string) Option {
	return func(o *BackupOpts) {
		o.TargetRecoveryFile = targetRecoveryFile
	}
}

func WithPitr(file string, targetTime *time.Time) Option {
	return func(o *BackupOpts) {
		o.PitrFile = file
		o.PitrTime = targetTime
	}
}

func WithUserEnv(u string) Option {
	return func(o *BackupOpts) {
		o.UserEnv = u
	}
}

func WithPasswordEnv(p string) Option {
	return func(o *BackupOpts) {
		o.PasswordEnv = p
	}
}

func WithDumpOpts(opts []string) Option {
	return func(o *BackupOpts) {
		o.DumpOpts = opts
	}
}

func New(userOpts ...Option) (Commander, error) {
	opts := &BackupOpts{}

	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.BackupPath == "" {
		return nil, errors.New("base path not provided")
	}
	if opts.UserEnv == "" {
		return nil, errors.New("user environment variable not provided")
	}
	if opts.PasswordEnv == "" {
		return nil, errors.New("password environment variable not provided")
	}

	return &logicalBackup{opts}, nil
}
