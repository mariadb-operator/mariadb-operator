package command

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestMariadbDumpArgs(t *testing.T) {
	tests := []struct {
		name      string
		backupCmd *BackupCommand
		backup    *mariadbv1alpha1.Backup
		mariadb   *mariadbv1alpha1.MariaDB
		wantArgs  []string
	}{
		{
			name:      "empty",
			backupCmd: &BackupCommand{},
			backup:    &mariadbv1alpha1.Backup{},
			mariadb:   &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
			},
		},
		{
			name: "extra args",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--verbose",
						"--add-drop-table",
					},
				},
			},
			backup:  &mariadbv1alpha1.Backup{},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--verbose",
				"--add-drop-table",
			},
		},
		{
			name:      "Galera",
			backupCmd: &BackupCommand{},
			backup:    &mariadbv1alpha1.Backup{},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--skip-add-locks",
			},
		},
		{
			name:      "ignore mysql.global_priv",
			backupCmd: &BackupCommand{},
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					IgnoreGlobalPriv: ptr.To(true),
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--ignore-table=mysql.global_priv",
			},
		},
		{
			name: "duplicated args",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--events",
						"--all-databases",
						"--skip-add-locks",
						"--ignore-table=mysql.global_priv",
						"--verbose",
						"--add-drop-table",
					},
				},
			},
			backup: &mariadbv1alpha1.Backup{},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--skip-add-locks",
				"--ignore-table=mysql.global_priv",
				"--verbose",
				"--add-drop-table",
			},
		},
		{
			name: "databases via args",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--databases db1 db2 db3",
					},
				},
			},
			backup:  &mariadbv1alpha1.Backup{},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		},
		{
			name:      "databases via spec.databases",
			backupCmd: &BackupCommand{},
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		},
		{
			name: "override databases via args with spec.databases",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--databases foo bar",
					},
				},
			},
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		},
		{
			name: "override malformed databases via args with spec.databases",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"      --databases    foo bar",
					},
				},
			},
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		},
		{
			name: "all",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--databases foo bar",
						"--verbose",
						"--add-drop-table",
					},
				},
			},
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
					IgnoreGlobalPriv: ptr.To(true),
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
				"--skip-add-locks",
				"--ignore-table=mysql.global_priv",
				"--verbose",
				"--add-drop-table",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.backupCmd.mariadbDumpArgs(tt.backup, tt.mariadb)
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("expecting args to be:\n%v\ngot:\n%v\n", tt.wantArgs, args)
			}
		})
	}
}

func TestMariadbArgs(t *testing.T) {
	tests := []struct {
		name      string
		backupCmd *BackupCommand
		restore   *mariadbv1alpha1.Restore
		wantArgs  []string
	}{
		{
			name:      "empty",
			backupCmd: &BackupCommand{},
			restore:   &mariadbv1alpha1.Restore{},
			wantArgs:  nil,
		},
		{
			name: "args",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					DumpOpts: []string{
						"--verbose",
						"--one-database db1",
					},
				},
			},
			restore: &mariadbv1alpha1.Restore{},
			wantArgs: []string{
				"--verbose",
				"--one-database db1",
			},
		},
		{
			name: "duplicate args",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					DumpOpts: []string{
						"--verbose",
						"--verbose",
						"--one-database db1",
					},
				},
			},
			restore: &mariadbv1alpha1.Restore{},
			wantArgs: []string{
				"--verbose",
				"--one-database db1",
			},
		},
		{
			name:      "database",
			backupCmd: &BackupCommand{},
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					Database: "db1",
				},
			},
			wantArgs: []string{
				"--one-database db1",
			},
		},
		{
			name: "database and args",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					DumpOpts: []string{
						"--verbose",
					},
				},
			},
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					Database: "db1",
				},
			},
			wantArgs: []string{
				"--verbose",
				"--one-database db1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.backupCmd.mariadbArgs(tt.restore)
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("expecting args to be:\n%v\ngot:\n%v\n", tt.wantArgs, args)
			}
		})
	}
}
