package backupcmd

import (
	"strings"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
)

type Command struct {
	Command []string
	Args    []string
}

func execCommand(args []string) *Command {
	return &Command{
		Command: []string{"sh", "-c"},
		Args:    []string{strings.Join(args, ";")},
	}
}

type Commander interface {
	BackupCommand(backup *databasev1alpha1.BackupMariaDB) *Command
	RestoreCommand() *Command
}
