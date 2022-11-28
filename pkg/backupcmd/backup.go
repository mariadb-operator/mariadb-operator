package backupcmd

import (
	"errors"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type BackupType int

const (
	Logical BackupType = iota
	Physical
)

type Option func(*CommandOpts)

func WithMariaDB(mdb *databasev1alpha1.MariaDB) Option {
	return func(co *CommandOpts) {
		co.MariaDB = mdb
	}
}

func WithBackupType(t BackupType) Option {
	return func(co *CommandOpts) {
		co.BackupType = t
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

func WithUser(u string) Option {
	return func(co *CommandOpts) {
		co.User = u
	}
}

func WithPassword(p string) Option {
	return func(co *CommandOpts) {
		co.Password = p
	}
}

func WithCleanup(c bool) Option {
	return func(co *CommandOpts) {
		co.Cleanup = c
	}
}

func WithHistory(h bool) Option {
	return func(co *CommandOpts) {
		co.History = h
	}
}

type CommandOpts struct {
	MariaDB    *databasev1alpha1.MariaDB
	BackupType BackupType
	BackupFile string
	BasePath   string
	User       string
	Password   string
	Cleanup    bool
	History    bool
}

func New(userOpts ...Option) (Commander, error) {
	opts := &CommandOpts{
		BackupType: Logical,
		Cleanup:    true,
		History:    true,
	}

	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.MariaDB == nil {
		return nil, errors.New("MariaDB not provided")
	}
	if opts.BasePath == "" {
		return nil, errors.New("base path not provided")
	}
	if opts.User == "" {
		return nil, errors.New("user not provided")
	}
	if opts.Password == "" {
		return nil, errors.New("password not provided")
	}

	var commander Commander
	if opts.BackupType == Physical {
		commander = &physicalBackup{opts}
	} else {
		commander = &logicalBackup{opts}
	}
	return commander, nil
}
