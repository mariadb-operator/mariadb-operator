package command

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

func TestMariadbDumpArgs(t *testing.T) {
	tests := []struct {
		name      string
		backupCmd *BackupCommand
		mariadb   *mariadbv1alpha1.MariaDB
		wantArgs  []string
	}{
		{
			name:      "emtpty",
			backupCmd: &BackupCommand{},
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
			name: "Galera with extra args",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--verbose",
						"--add-drop-table",
					},
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
				"--all-databases",
				"--skip-add-locks",
				"--verbose",
				"--add-drop-table",
			},
		},
		{
			name: "Duplicated args",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--events",
						"--all-databases",
						"--skip-add-locks",
						"--verbose",
						"--add-drop-table",
					},
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
				"--all-databases",
				"--skip-add-locks",
				"--verbose",
				"--add-drop-table",
			},
		},
		{
			name: "Explicit databases",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--databases foo bar",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases foo bar",
			},
		},
		{
			name: "Explicit databases with extra args",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--databases foo bar",
						"--verbose",
						"--add-drop-table",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases foo bar",
				"--verbose",
				"--add-drop-table",
			},
		},
		{
			name: "All",
			backupCmd: &BackupCommand{
				BackupOpts{
					DumpOpts: []string{
						"--databases foo bar",
						"--verbose",
						"--add-drop-table",
					},
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
				"--skip-add-locks",
				"--databases foo bar",
				"--verbose",
				"--add-drop-table",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.backupCmd.mariadbDumpArgs(tt.mariadb)
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("expecting args to be:\n%v\ngot:\n%v\n", tt.wantArgs, args)
			}
		})
	}
}
