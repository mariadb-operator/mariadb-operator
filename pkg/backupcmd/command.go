package backupcmd

import (
	"fmt"
	"strings"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type Command struct {
	Command []string
	Args    []string
}

type Commander interface {
	BackupCommand(backup *databasev1alpha1.BackupMariaDB) *Command
	RestoreCommand() *Command
}

func execCommand(args []string) *Command {
	return &Command{
		Command: []string{"sh", "-c"},
		Args:    []string{strings.Join(args, ";")},
	}
}

func authFlags(co *CommandOpts) string {
	return fmt.Sprintf(
		"--user=${%s} --password=${%s} --host=%s --port=%d",
		co.UserEnv,
		co.PasswordEnv,
		co.MariaDB.Name,
		co.MariaDB.Spec.Port,
	)
}
