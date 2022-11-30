package backupcmd

import (
	"errors"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type Option func(*CommandOpts)

func WithMariaDB(mdb *databasev1alpha1.MariaDB) Option {
	return func(co *CommandOpts) {
		co.MariaDB = mdb
	}
}

func WithBackupPhysical(p bool) Option {
	return func(co *CommandOpts) {
		co.BackupPhysical = p
	}
}

func WithFile(f string) Option {
	return func(co *CommandOpts) {
		co.BackupFile = f
	}
}

func WithBasePath(p string) Option {
	return func(co *CommandOpts) {
		co.BasePath = p
	}
}

func WithUserEnv(u string) Option {
	return func(co *CommandOpts) {
		co.UserEnv = u
	}
}

func WithPasswordEnv(p string) Option {
	return func(co *CommandOpts) {
		co.PasswordEnv = p
	}
}

type CommandOpts struct {
	MariaDB        *databasev1alpha1.MariaDB
	BackupPhysical bool
	BackupFile     string
	BasePath       string
	UserEnv        string
	PasswordEnv    string
}

func New(userOpts ...Option) (Commander, error) {
	opts := &CommandOpts{}

	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.MariaDB == nil {
		return nil, errors.New("MariaDB not provided")
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

	var commander Commander
	if opts.BackupPhysical {
		commander = &physicalBackup{opts}
	} else {
		commander = &logicalBackup{opts}
	}
	return commander, nil
}
