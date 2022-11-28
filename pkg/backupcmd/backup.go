package backupcmd

import (
	"errors"
)

type Option func(*CommandOpts)

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
