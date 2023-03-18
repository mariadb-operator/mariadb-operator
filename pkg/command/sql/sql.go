package sql

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
)

type SqlOpts struct {
	command.CommandOpts
	SqlFile string
}

type SqlCommand struct {
	*SqlOpts
}

func (s *SqlCommand) ExecCommand(mariadb *mariadbv1alpha1.MariaDB) *command.Command {
	cmds := []string{
		"echo '⚙️ Executing SQL script'",
		fmt.Sprintf(
			"mysql %s  < %s",
			command.ConnectionFlags(&s.SqlOpts.CommandOpts, mariadb),
			s.SqlFile,
		),
	}
	return command.ExecCommand(cmds)
}

type Option func(*SqlOpts)

func WithUserEnv(u string) Option {
	return func(so *SqlOpts) {
		so.UserEnv = u
	}
}

func WithPasswordEnv(p string) Option {
	return func(so *SqlOpts) {
		so.PasswordEnv = p
	}
}

func WithDatabase(d string) Option {
	return func(so *SqlOpts) {
		so.Database = &d
	}
}

func WithSqlFile(f string) Option {
	return func(so *SqlOpts) {
		so.SqlFile = f
	}
}

func New(userOpts ...Option) (*SqlCommand, error) {
	opts := &SqlOpts{}

	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.UserEnv == "" {
		return nil, errors.New("user environment variable not provided")
	}
	if opts.PasswordEnv == "" {
		return nil, errors.New("password environment variable not provided")
	}
	if opts.SqlFile == "" {
		return nil, errors.New("sql file not provided")
	}

	return &SqlCommand{opts}, nil
}
