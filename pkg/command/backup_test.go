package command

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/pkg/builder/pki"
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
					ExtraOpts: []string{
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
			name:      "TLS",
			backupCmd: &BackupCommand{},
			backup:    &mariadbv1alpha1.Backup{},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			wantArgs: []string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--ssl",
				"--ssl-ca",
				builderpki.CACertPath,
				"--ssl-cert",
				builderpki.ClientCertPath,
				"--ssl-key",
				builderpki.ClientKeyPath,
				"--ssl-verify-server-cert",
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
					ExtraOpts: []string{
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
					ExtraOpts: []string{
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
					ExtraOpts: []string{
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
					ExtraOpts: []string{
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
					ExtraOpts: []string{
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
			if diff := cmp.Diff(args, tt.wantArgs); diff != "" {
				t.Errorf("unexpected args (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMariadbOperatorBackup(t *testing.T) {
	tests := []struct {
		name      string
		backupCmd *BackupCommand
		wantArgs  []string
	}{
		{
			name: "no S3 no cleanupTargetFile",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
				},
			},
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--max-retention",
				"24h0m0s",
				"--compression",
				"gzip",
				"--log-level",
				"info",
			},
		},
		{
			name: "S3",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
					S3:                   true,
					S3Bucket:             "backups",
					S3Endpoint:           "s3.amazonaws.com",
					S3Region:             "us-east-1",
					S3TLS:                true,
					S3CACertPath:         "/etc/ssl/ca.crt",
					S3Prefix:             "mariadb",
				},
			},
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--max-retention",
				"24h0m0s",
				"--compression",
				"gzip",
				"--log-level",
				"info",
				"--s3",
				"--s3-bucket",
				"backups",
				"--s3-endpoint",
				"s3.amazonaws.com",
				"--s3-region",
				"us-east-1",
				"--s3-tls",
				"--s3-ca-cert-path",
				"/etc/ssl/ca.crt",
				"--s3-prefix",
				"mariadb",
			},
		},
		{
			name: "S3 and cleanup target file",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
					S3:                   true,
					S3Bucket:             "backups",
					S3Endpoint:           "s3.amazonaws.com",
					S3Region:             "us-east-1",
					S3TLS:                true,
					S3CACertPath:         "/etc/ssl/ca.crt",
					S3Prefix:             "mariadb",
					CleanupTargetFile:    true,
				},
			},
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--max-retention",
				"24h0m0s",
				"--compression",
				"gzip",
				"--log-level",
				"info",
				"--s3",
				"--s3-bucket",
				"backups",
				"--s3-endpoint",
				"s3.amazonaws.com",
				"--s3-region",
				"us-east-1",
				"--s3-tls",
				"--s3-ca-cert-path",
				"/etc/ssl/ca.crt",
				"--s3-prefix",
				"mariadb",
				"--cleanup-target-file",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := tt.backupCmd.MariadbOperatorBackup()
			if diff := cmp.Diff(command.Args, tt.wantArgs); diff != "" {
				t.Errorf("unexpected args (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMariadbArgs(t *testing.T) {
	tests := []struct {
		name      string
		backupCmd *BackupCommand
		restore   *mariadbv1alpha1.Restore
		mariadb   *mariadbv1alpha1.MariaDB
		wantArgs  []string
	}{
		{
			name:      "empty",
			backupCmd: &BackupCommand{},
			restore:   &mariadbv1alpha1.Restore{},
			mariadb:   &mariadbv1alpha1.MariaDB{},
			wantArgs:  nil,
		},
		{
			name: "args",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{
						"--verbose",
						"--one-database db1",
					},
				},
			},
			restore: &mariadbv1alpha1.Restore{},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--verbose",
				"--one-database db1",
			},
		},
		{
			name: "duplicate args",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{
						"--verbose",
						"--verbose",
						"--one-database db1",
					},
				},
			},
			restore: &mariadbv1alpha1.Restore{},
			mariadb: &mariadbv1alpha1.MariaDB{},
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
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--one-database db1",
			},
		},
		{
			name: "database and args",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{
						"--verbose",
					},
				},
			},
			restore: &mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					Database: "db1",
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantArgs: []string{
				"--verbose",
				"--one-database db1",
			},
		},
		{
			name:      "TLS",
			backupCmd: &BackupCommand{},
			restore:   &mariadbv1alpha1.Restore{},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			wantArgs: []string{
				"--ssl",
				"--ssl-ca",
				builderpki.CACertPath,
				"--ssl-cert",
				builderpki.ClientCertPath,
				"--ssl-key",
				builderpki.ClientKeyPath,
				"--ssl-verify-server-cert",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.backupCmd.mariadbArgs(tt.restore, tt.mariadb)
			if diff := cmp.Diff(args, tt.wantArgs); diff != "" {
				t.Errorf("unexpected args (-want +got):\n%s", diff)
			}
		})
	}
}
