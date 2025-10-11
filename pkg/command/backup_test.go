package command

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestNewBackupCommand(t *testing.T) {
	tests := []struct {
		name    string
		opts    []BackupOpt
		wantErr bool
	}{
		{
			name: "missing path",
			opts: []BackupOpt{
				WithBackup("", "/target/file"),
				WithBackupUserEnv("USER_ENV"),
				WithBackupPasswordEnv("PASS_ENV"),
			},
			wantErr: true,
		},
		{
			name: "missing target file",
			opts: []BackupOpt{
				WithBackup("/backups", ""),
				WithBackupUserEnv("USER_ENV"),
				WithBackupPasswordEnv("PASS_ENV"),
			},
			wantErr: true,
		},
		{
			name: "missing user env",
			opts: []BackupOpt{
				WithBackup("/backups", "/target/file"),
				WithBackupPasswordEnv("PASS_ENV"),
			},
			wantErr: true,
		},
		{
			name: "missing password env",
			opts: []BackupOpt{
				WithBackup("/backups", "/target/file"),
				WithBackupUserEnv("USER_ENV"),
			},
			wantErr: true,
		},
		{
			name: "omit credentials skips user/password check",
			opts: []BackupOpt{
				WithBackup("/backups", "/target/file"),
				WithOmitCredentials(true),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewBackupCommand(tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cmd)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

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

func TestMariadbBackupArgs(t *testing.T) {
	tests := []struct {
		name           string
		backupCmd      *BackupCommand
		mariadb        *mariadbv1alpha1.MariaDB
		targetPodIndex int
		wantArgs       []string
	}{
		{
			name:           "default",
			backupCmd:      &BackupCommand{},
			mariadb:        &mariadbv1alpha1.MariaDB{},
			targetPodIndex: 0,
			wantArgs: []string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
			},
		},
		{
			name: "with extra opts",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{"--compress", "--parallel=2"},
				},
			},
			mariadb:        &mariadbv1alpha1.MariaDB{},
			targetPodIndex: 0,
			wantArgs: []string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--compress",
				"--parallel=2",
			},
		},
		{
			name:      "with TLS",
			backupCmd: &BackupCommand{},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{Enabled: true},
				},
			},
			targetPodIndex: 0,
			wantArgs: []string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
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
			name: "with TLS and extra opts",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{"--compress", "--parallel=2"},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{Enabled: true},
				},
			},
			targetPodIndex: 0,
			wantArgs: []string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--ssl",
				"--ssl-ca",
				builderpki.CACertPath,
				"--ssl-cert",
				builderpki.ClientCertPath,
				"--ssl-key",
				builderpki.ClientKeyPath,
				"--ssl-verify-server-cert",
				"--compress",
				"--parallel=2",
			},
		},
		{
			name: "duplicate extra opts",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{"--compress", "--compress"},
				},
			},
			mariadb:        &mariadbv1alpha1.MariaDB{},
			targetPodIndex: 0,
			wantArgs: []string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--compress",
			},
		},
		{
			name:      "replication",
			backupCmd: &BackupCommand{},
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			targetPodIndex: 0,
			wantArgs: []string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--slave-info",
				"--safe-slave-backup",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.backupCmd.mariadbBackupArgs(tt.mariadb, tt.targetPodIndex)
			if diff := cmp.Diff(args, tt.wantArgs); diff != "" {
				t.Errorf("unexpected args (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMariadbOperatorBackup(t *testing.T) {
	tests := []struct {
		name              string
		backupCmd         *BackupCommand
		backupContentType mariadbv1alpha1.BackupContentType
		wantArgs          []string
	}{
		{
			name: "logical no S3 no cleanupTargetFile",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
				},
			},
			backupContentType: mariadbv1alpha1.BackupContentTypeLogical,
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--backup-content-type",
				string(mariadbv1alpha1.BackupContentTypeLogical),
				"--max-retention",
				"24h0m0s",
				"--compression",
				"gzip",
				"--log-level",
				"info",
			},
		},
		{
			name: "physical no S3 no cleanupTargetFile",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
				},
			},
			backupContentType: mariadbv1alpha1.BackupContentTypePhysical,
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--backup-content-type",
				string(mariadbv1alpha1.BackupContentTypePhysical),
				"--max-retention",
				"24h0m0s",
				"--compression",
				"gzip",
				"--log-level",
				"info",
			},
		},
		{
			name: "logical S3",
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
			backupContentType: mariadbv1alpha1.BackupContentTypeLogical,
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--backup-content-type",
				string(mariadbv1alpha1.BackupContentTypeLogical),
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
			name: "physical S3",
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
			backupContentType: mariadbv1alpha1.BackupContentTypePhysical,
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--backup-content-type",
				string(mariadbv1alpha1.BackupContentTypePhysical),
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
			name: "logical S3 and cleanup target file",
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
			backupContentType: mariadbv1alpha1.BackupContentTypeLogical,
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--backup-content-type",
				string(mariadbv1alpha1.BackupContentTypeLogical),
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
		{
			name: "physical S3 and cleanup target file",
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
			backupContentType: mariadbv1alpha1.BackupContentTypePhysical,
			wantArgs: []string{
				"backup",
				"--path",
				"/backups",
				"--target-file-path",
				"/backups/0-backup-target.txt",
				"--backup-content-type",
				string(mariadbv1alpha1.BackupContentTypePhysical),
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
			command := tt.backupCmd.MariadbOperatorBackup(tt.backupContentType)
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

func TestMariadbBackupRestore(t *testing.T) {
	tests := []struct {
		name        string
		backupCmd   *BackupCommand
		mariadb     *mariadbv1alpha1.MariaDB
		backupDir   string
		restoreOpts []MariaDBBackupRestoreOpt
		wantErr     bool
		wantCleanup bool
	}{
		{
			name: "with database option (should error)",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					CommandOpts: CommandOpts{
						Database: ptr.To("somedb"),
					},
					TargetFilePath: "/backups/target.sql",
				},
			},
			mariadb:     &mariadbv1alpha1.MariaDB{},
			backupDir:   "/backup/full",
			restoreOpts: nil,
			wantErr:     true,
			wantCleanup: false,
		},
		{
			name: "basic physical restore",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					TargetFilePath: "/backups/target.sql",
				},
			},
			mariadb:     &mariadbv1alpha1.MariaDB{},
			backupDir:   "/backup/full",
			restoreOpts: nil,
			wantErr:     false,
			wantCleanup: false,
		},
		{
			name: "with cleanup data dir (should include cleanup command)",
			backupCmd: &BackupCommand{
				BackupOpts: BackupOpts{
					TargetFilePath: "/backups/target.sql",
				},
			},
			mariadb:     &mariadbv1alpha1.MariaDB{},
			backupDir:   "/backup/full",
			restoreOpts: []MariaDBBackupRestoreOpt{WithCleanupDataDir(true)},
			wantErr:     false,
			wantCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.backupCmd.MariadbBackupRestore(tt.mariadb, tt.backupDir, tt.restoreOpts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cmd)
				return
			}

			assert.NoError(t, err)
			if assert.NotNil(t, cmd) {
				if len(cmd.Args) == 0 {
					t.Error("Expected command to return args")
				}
				script := cmd.Args[0] // NewBashCommand puts the whole script here
				if tt.wantCleanup {
					assert.Contains(t, script, "rm -rf /var/lib/mysql/*")
				} else {
					assert.NotContains(t, script, "rm -rf /var/lib/mysql/*")
				}
			}
		})
	}
}

func TestExistingBackupRestoreCmd(t *testing.T) {
	tests := []struct {
		name              string
		backupDirPath     string
		cleanupDataDirCmd string
		copyBackupCmd     string
		copyBinlogCmds    []string
		restoreOpts       []MariaDBBackupRestoreOpt
		wantCleanup       bool
	}{
		{
			name:              "no cleanup no binlogs",
			backupDirPath:     "/backup/full",
			cleanupDataDirCmd: `if [ -d /var/lib/mysql ]; then echo "cleanup"; rm -rf /var/lib/mysql/*; fi`,
			copyBackupCmd:     "mariadb-backup --copy-back --target-dir=/backup/full --force-non-empty-directories",
			copyBinlogCmds:    []string{},
			restoreOpts:       nil,
			wantCleanup:       false,
		},
		{
			name:              "with cleanup and one binlog",
			backupDirPath:     "/backup/full",
			cleanupDataDirCmd: `if [ -d /var/lib/mysql ]; then echo "cleanup"; rm -rf /var/lib/mysql/*; fi`,
			copyBackupCmd:     "mariadb-backup --copy-back --target-dir=/backup/full --force-non-empty-directories",
			copyBinlogCmds:    []string{"cp /backup/full/binlog1 /var/lib/mysql/mariadb_operator_position"},
			restoreOpts:       []MariaDBBackupRestoreOpt{WithCleanupDataDir(true)},
			wantCleanup:       true,
		},
		{
			name:              "no cleanup with two binlogs",
			backupDirPath:     "/another/restore",
			cleanupDataDirCmd: `if [ -d /var/lib/mysql ]; then echo "cleanup"; rm -rf /var/lib/mysql/*; fi`,
			copyBackupCmd:     "mariadb-backup --copy-back --target-dir=/another/restore --force-non-empty-directories",
			copyBinlogCmds: []string{
				"cp /another/restore/binlogA /var/lib/mysql/mariadb_operator_position",
				"cp /another/restore/binlogB /var/lib/mysql/mariadb_operator_position",
			},
			restoreOpts: nil,
			wantCleanup: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			b := &BackupCommand{}
			out, err := b.existingBackupRestoreCmd(
				tt.backupDirPath,
				tt.cleanupDataDirCmd,
				tt.copyBackupCmd,
				tt.copyBinlogCmds,
				tt.restoreOpts...,
			)
			assert.NoError(t, err)
			assert.NotEmpty(t, out)

			assert.Contains(t, out, "if [ -d "+tt.backupDirPath+" ]; then")
			assert.Contains(t, out, tt.copyBackupCmd)
			assert.Contains(t, out, "exit 0")

			if tt.wantCleanup {
				assert.Contains(t, out, "rm -rf /var/lib/mysql/*")
			} else {
				assert.NotContains(t, out, "rm -rf /var/lib/mysql/*")
			}

			for _, cmd := range tt.copyBinlogCmds {
				assert.Contains(t, out, cmd)
			}
		})
	}
}
