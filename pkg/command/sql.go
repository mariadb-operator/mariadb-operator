package command

import (
	"errors"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/interfaces"
)

type SqlOpts struct {
	CommandOpts
	SSLCAPath   *string
	SSLCertPath *string
	SSLKeyPath  *string
	SqlFile     string
}

type SqlOpt func(*SqlOpts)

func WithSqlFile(f string) SqlOpt {
	return func(so *SqlOpts) {
		so.SqlFile = f
	}
}

func WithSqlUserEnv(u string) SqlOpt {
	return func(so *SqlOpts) {
		so.UserEnv = u
	}
}

func WithSqlPasswordEnv(p string) SqlOpt {
	return func(so *SqlOpts) {
		so.PasswordEnv = p
	}
}

func WithSqlDatabase(d string) SqlOpt {
	return func(so *SqlOpts) {
		so.Database = &d
	}
}

func WithSSL(caPath, certPath, keyPath string) SqlOpt {
	return func(o *SqlOpts) {
		o.SSLCAPath = &caPath
		o.SSLCertPath = &certPath
		o.SSLKeyPath = &keyPath
	}
}

type SqlCommand struct {
	*SqlOpts
}

func (s *SqlCommand) ExecCommand(mariadb interfaces.ConnectionParamsAwareInterface) (*Command, error) {
	sqlFlags, err := s.SqlFlags(mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting SQL flags: %v", err)
	}

	cmds := []string{
		"set -euo pipefail",
		"echo '⚙️ Executing SQL script'",
		fmt.Sprintf(
			"mariadb %s < %s",
			sqlFlags,
			s.SqlFile,
		),
	}
	return NewBashCommand(cmds), nil
}

func (s *SqlCommand) SqlFlags(mdb interfaces.ConnectionParamsAwareInterface) (string, error) {
	flags, err := ConnectionFlags(&s.SqlOpts.CommandOpts, mdb)
	if err != nil {
		return "", fmt.Errorf("error getting connection flags: %v", err)
	}

	if s.SSLCAPath != nil && s.SSLCertPath != nil && s.SSLKeyPath != nil {
		flags += fmt.Sprintf(" --ssl --ssl-ca=%s --ssl-cert=%s --ssl-key=%s --ssl-verify-server-cert",
			*s.SSLCAPath, *s.SSLCertPath, *s.SSLKeyPath)
	}
	return flags, nil
}

func NewSqlCommand(userOpts ...SqlOpt) (*SqlCommand, error) {
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
