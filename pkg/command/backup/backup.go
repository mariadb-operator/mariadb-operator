package backup

import (
	"errors"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
)

type Commander interface {
	BackupCommand(backup *mariadbv1alpha1.Backup, mariadb *mariadbv1alpha1.MariaDB) *command.Command
	RestoreCommand(mariadb *mariadbv1alpha1.MariaDB) *command.Command
}

type BackupOpts struct {
	command.CommandOpts
	DumpOpts   string
	BackupFile string
	BasePath   string
}

type Option func(*BackupOpts)

func WithFile(f string) Option {
	return func(co *BackupOpts) {
		co.BackupFile = f
	}
}

func WithBasePath(p string) Option {
	return func(co *BackupOpts) {
		co.BasePath = p
	}
}

func WithUserEnv(u string) Option {
	return func(co *BackupOpts) {
		co.UserEnv = u
	}
}

func WithPasswordEnv(p string) Option {
	return func(co *BackupOpts) {
		co.PasswordEnv = p
	}
}

func New(userOpts ...Option) (Commander, error) {
	opts := &BackupOpts{}

	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.BasePath == "" {
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
