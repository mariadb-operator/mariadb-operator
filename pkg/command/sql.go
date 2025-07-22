package command

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

type SQLOpts struct {
	CommandOpts
	SSLCAPath   *string
	SSLCertPath *string
	SSLKeyPath  *string
	SQLFile     string
}

type SQLOpt func(*SQLOpts)

func WithSQLFile(f string) SQLOpt {
	return func(so *SQLOpts) {
		so.SQLFile = f
	}
}

func WithSQLUserEnv(u string) SQLOpt {
	return func(so *SQLOpts) {
		so.UserEnv = u
	}
}

func WithSQLPasswordEnv(p string) SQLOpt {
	return func(so *SQLOpts) {
		so.PasswordEnv = p
	}
}

func WithSQLDatabase(d string) SQLOpt {
	return func(so *SQLOpts) {
		so.Database = &d
	}
}

func WithSSL(caPath, certPath, keyPath string) SQLOpt {
	return func(o *SQLOpts) {
		o.SSLCAPath = &caPath
		o.SSLCertPath = &certPath
		o.SSLKeyPath = &keyPath
	}
}

type SQLCommand struct {
	*SQLOpts
}

func (s *SQLCommand) ExecCommand(mariadb *mariadbv1alpha1.MariaDB) (*Command, error) {
	sqlFlags, err := s.SQLFlags(mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting SQL flags: %v", err)
	}
	cmds := []string{
		"set -euo pipefail",
		"echo '⚙️ Executing SQL script'",
		fmt.Sprintf(
			"mariadb %s < %s",
			sqlFlags,
			s.SQLFile,
		),
	}
	return NewBashCommand(cmds), nil
}

func (s *SQLCommand) SQLFlags(mdb *mariadbv1alpha1.MariaDB) (string, error) {
	flags, err := ConnectionFlags(&s.CommandOpts, mdb)
	if err != nil {
		return "", fmt.Errorf("error getting connection flags: %v", err)
	}
	if s.SSLCAPath != nil && s.SSLCertPath != nil && s.SSLKeyPath != nil {
		flags += fmt.Sprintf(" --ssl --ssl-ca=%s --ssl-cert=%s --ssl-key=%s --ssl-verify-server-cert",
			*s.SSLCAPath, *s.SSLCertPath, *s.SSLKeyPath)
	}
	return flags, nil
}

func NewSQLCommand(userOpts ...SQLOpt) (*SQLCommand, error) {
	opts := &SQLOpts{}

	for _, setOpt := range userOpts {
		setOpt(opts)
	}
	if opts.UserEnv == "" {
		return nil, errors.New("user environment variable not provided")
	}
	if opts.PasswordEnv == "" {
		return nil, errors.New("password environment variable not provided")
	}
	if opts.SQLFile == "" {
		return nil, errors.New("sql file not provided")
	}

	return &SQLCommand{opts}, nil
}
