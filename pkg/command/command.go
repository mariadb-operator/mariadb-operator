package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/interfaces"
)

type Command struct {
	Command []string
	Args    []string
}

type CommandOpts struct {
	UserEnv     string
	PasswordEnv string
	Database    *string
}

func NewCommand(cmd, args []string) *Command {
	return &Command{
		Command: cmd,
		Args:    args,
	}
}

func NewBashCommand(args []string) *Command {
	return &Command{
		Command: []string{"bash", "-c"},
		Args:    []string{strings.Join(args, ";")},
	}
}

type ConnectionFlagsOpts struct {
	Host string
}

type ConnectionFlagOpt func(*ConnectionFlagsOpts)

func WithHostConnectionFlag(host string) ConnectionFlagOpt {
	return func(o *ConnectionFlagsOpts) {
		o.Host = host
	}
}

func ConnectionFlags(co *CommandOpts, mariadb interfaces.Connector,
	connectionFlagOpts ...ConnectionFlagOpt) (string, error) {
	opts := &ConnectionFlagsOpts{}
	for _, setOpt := range connectionFlagOpts {
		setOpt(opts)
	}

	if co.UserEnv == "" {
		return "", errors.New("UserEnv must be set")
	}
	if co.PasswordEnv == "" {
		return "", errors.New("PasswordEnv must be set")
	}

	flags := fmt.Sprintf(
		"--user=${%s} --password=${%s} --host=%s --port=%d",
		co.UserEnv,
		co.PasswordEnv,
		host(mariadb, opts),
		mariadb.GetPort(),
	)
	if co.Database != nil {
		flags += fmt.Sprintf(" --database=%s", *co.Database)
	}
	return flags, nil
}

func host(mariadb interfaces.Connector, opts *ConnectionFlagsOpts) string {
	if opts.Host != "" {
		return opts.Host
	}
	return mariadb.GetHost()
}
func PodConnectionFlags(co *CommandOpts, mariadb interfaces.Connector, podIndex int) (string, error) {

	if co.UserEnv == "" {
		return "", errors.New("UserEnv must be set")
	}
	if co.PasswordEnv == "" {
		return "", errors.New("PasswordEnv must be set")
	}

	flags := fmt.Sprintf(
		"--user=${%s} --password=${%s} --host=%s --port=%d",
		co.UserEnv,
		co.PasswordEnv,
		mariadb.GetPodHost(podIndex),
		mariadb.GetPort(),
	)
	if co.Database != nil {
		flags += fmt.Sprintf(" --database=%s", *co.Database)
	}
	return flags, nil
}
