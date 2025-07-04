package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mariadb-operator/mariadb-operator/pkg/interfaces"
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

func ConnectionFlags(co *CommandOpts, mariadb interfaces.ConnectionParamsAwareInterface) (string, error) {

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
		mariadb.GetHost(),
		mariadb.GetPort(),
	)
	if co.Database != nil {
		flags += fmt.Sprintf(" --database=%s", *co.Database)
	}
	return flags, nil
}

// func host(mariadb interfaces.MariaDBGenericInterface) string {

// 	if mariadb.GetObjectKind().GroupVersionKind().Kind == mariadbv1alpha1.ExternalMariaDBKind {
// 		return mariadb.GetHost()
// 	}
// 	mariadbObj := mariadb.(*mariadbv1alpha1.MariaDB)
// 	if mariadb.IsHAEnabled() {
// 		return statefulset.ServiceFQDNWithService(
// 			mariadbObj.ObjectMeta,
// 			mariadbObj.PrimaryServiceKey().Name,
// 		)
// 	}
// 	return statefulset.ServiceFQDN(mariadbObj.ObjectMeta)
// }
