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
	BackupCommand(backup *databasev1alpha1.BackupMariaDB, mariadb *databasev1alpha1.MariaDB) *Command
	RestoreCommand(mariadb *databasev1alpha1.MariaDB) *Command
}

func execCommand(args []string) *Command {
	return &Command{
		Command: []string{"sh", "-c"},
		Args:    []string{strings.Join(args, ";")},
	}
}

func authFlags(co *CommandOpts, mariadb *databasev1alpha1.MariaDB) string {
	return fmt.Sprintf(
		"--user=${%s} --password=${%s} --host=%s --port=%d",
		co.UserEnv,
		co.PasswordEnv,
		mariadb.Name,
		mariadb.Spec.Port,
	)
}
