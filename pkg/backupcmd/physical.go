package backupcmd

import databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"

type physicalBackup struct {
	*CommandOpts
}

func (l *physicalBackup) BackupCommand(backup *databasev1alpha1.BackupMariaDB) *Command {
	return nil
}

func (l *physicalBackup) RestoreCommand() *Command {
	return nil
}
