package command

import (
	"fmt"
	"strings"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
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

func ConnectionFlags(co *CommandOpts, mariadb *mariadbv1alpha1.MariaDB) string {
	flags := fmt.Sprintf(
		"--user=${%s} --password=${%s} --host=%s --port=%d",
		co.UserEnv,
		co.PasswordEnv,
		host(mariadb),
		mariadb.Spec.Port,
	)
	if co.Database != nil {
		flags += fmt.Sprintf(" --database=%s", *co.Database)
	}
	return flags
}

func host(mariadb *mariadbv1alpha1.MariaDB) string {
	if mariadb.Replication().Enabled {
		return statefulset.ServiceFQDNWithService(
			mariadb.ObjectMeta,
			mariadb.PrimaryServiceKey().Name,
		)
	}
	return statefulset.ServiceFQDN(mariadb.ObjectMeta)
}
