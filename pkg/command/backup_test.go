package command

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var (
	startGtid  = mustParseGtid("0-10-1")
	targetTime = time.Now()

	mdbObjectMeta = metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test",
	}
	mdbFlags = "--user=${test} --password=${test} --host=test-primary.test.svc.cluster.local --port=3306"
	tlsFlags = "--ssl --ssl-ca /etc/pki/ca.crt --ssl-cert /etc/pki/client.crt --ssl-key /etc/pki/client.key --ssl-verify-server-cert"
)

var _ = Describe("NewBackupCommand", func() {
	DescribeTable("validates options",
		func(opts []BackupOpt, wantErr bool) {
			cmd, err := NewBackupCommand(opts...)
			if wantErr {
				Expect(err).To(HaveOccurred())
				Expect(cmd).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(cmd).NotTo(BeNil())
			}
		},
		Entry("missing path",
			[]BackupOpt{
				WithPath("", "/target/file", "/backup/full"),
				WithUserEnv("USER_ENV"),
				WithPasswordEnv("PASS_ENV"),
			},
			true,
		),
		Entry("missing target file",
			[]BackupOpt{
				WithPath("/backups", "", "/backup/full"),
				WithUserEnv("USER_ENV"),
				WithPasswordEnv("PASS_ENV"),
			},
			true,
		),
		Entry("missing backup full dir",
			[]BackupOpt{
				WithPath("/backups", "/target/file", ""),
				WithUserEnv("USER_ENV"),
				WithPasswordEnv("PASS_ENV"),
			},
			true,
		),
		Entry("missing user env",
			[]BackupOpt{
				WithPath("/backups", "/target/file", "/backup/full"),
				WithPasswordEnv("PASS_ENV"),
			},
			true,
		),
		Entry("missing password env",
			[]BackupOpt{
				WithPath("/backups", "/target/file", "/backup/full"),
				WithUserEnv("USER_ENV"),
			},
			true,
		),
		Entry("omit credentials skips user/password check",
			[]BackupOpt{
				WithPath("/backups", "/target/file", "/backup/full"),
				WithOmitCredentials(true),
			},
			false,
		),
	)
})

var _ = Describe("mariadbDumpArgs", func() {
	DescribeTable("builds mariadb-dump args",
		func(backupCmd *BackupCommand, backup *mariadbv1alpha1.Backup, mariadb *mariadbv1alpha1.MariaDB, wantArgs []string) {
			args := backupCmd.mariadbDumpArgs(backup, mariadb)
			Expect(args).To(Equal(wantArgs))
		},
		Entry("empty",
			&BackupCommand{},
			&mariadbv1alpha1.Backup{},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
			},
		),
		Entry("extra args",
			&BackupCommand{
				BackupOpts{
					ExtraOpts: []string{
						"--verbose",
						"--add-drop-table",
					},
				},
			},
			&mariadbv1alpha1.Backup{},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--verbose",
				"--add-drop-table",
			},
		),
		Entry("Galera",
			&BackupCommand{},
			&mariadbv1alpha1.Backup{},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--skip-add-locks",
			},
		),
		Entry("TLS",
			&BackupCommand{},
			&mariadbv1alpha1.Backup{},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			[]string{
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
		),
		Entry("ignore mysql.global_priv",
			&BackupCommand{},
			&mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					IgnoreGlobalPriv: ptr.To(true),
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--ignore-table=mysql.global_priv",
			},
		),
		Entry("duplicated args",
			&BackupCommand{
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
			&mariadbv1alpha1.Backup{},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--skip-add-locks",
				"--ignore-table=mysql.global_priv",
				"--verbose",
				"--add-drop-table",
			},
		),
		Entry("databases via args",
			&BackupCommand{
				BackupOpts{
					ExtraOpts: []string{
						"--databases db1 db2 db3",
					},
				},
			},
			&mariadbv1alpha1.Backup{},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		),
		Entry("databases via spec.databases",
			&BackupCommand{},
			&mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		),
		Entry("override databases via args with spec.databases",
			&BackupCommand{
				BackupOpts{
					ExtraOpts: []string{
						"--databases foo bar",
					},
				},
			},
			&mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		),
		Entry("override malformed databases via args with spec.databases",
			&BackupCommand{
				BackupOpts{
					ExtraOpts: []string{
						"      --databases    foo bar",
					},
				},
			},
			&mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
			},
		),
		Entry("all",
			&BackupCommand{
				BackupOpts{
					ExtraOpts: []string{
						"--databases foo bar",
						"--verbose",
						"--add-drop-table",
					},
				},
			},
			&mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Databases: []string{
						"db1",
						"db2",
						"db3",
					},
					IgnoreGlobalPriv: ptr.To(true),
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases db1 db2 db3",
				"--skip-add-locks",
				"--ignore-table=mysql.global_priv",
				"--verbose",
				"--add-drop-table",
			},
		),
	)
})

var _ = Describe("mariadbBackupArgs", func() {
	DescribeTable("builds mariadb-backup args",
		func(backupCmd *BackupCommand, mariadb *mariadbv1alpha1.MariaDB, targetPodIndex int, wantArgs []string) {
			args := backupCmd.mariadbBackupArgs(mariadb, targetPodIndex)
			Expect(args).To(Equal(wantArgs))
		},
		Entry("default",
			&BackupCommand{},
			&mariadbv1alpha1.MariaDB{},
			0,
			[]string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
			},
		),
		Entry("with extra opts",
			&BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{"--compress", "--parallel=2"},
				},
			},
			&mariadbv1alpha1.MariaDB{},
			0,
			[]string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--compress",
				"--parallel=2",
			},
		),
		Entry("with TLS",
			&BackupCommand{},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{Enabled: true},
				},
			},
			0,
			[]string{
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
		),
		Entry("with TLS and extra opts",
			&BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{"--compress", "--parallel=2"},
				},
			},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{Enabled: true},
				},
			},
			0,
			[]string{
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
		),
		Entry("duplicate extra opts",
			&BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{"--compress", "--compress"},
				},
			},
			&mariadbv1alpha1.MariaDB{},
			0,
			[]string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--compress",
			},
		),
		Entry("replication",
			&BackupCommand{},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
				Status: mariadbv1alpha1.MariaDBStatus{
					CurrentPrimaryPodIndex: ptr.To(1),
				},
			},
			0,
			[]string{
				"--backup",
				"--stream=xbstream",
				"--databases-exclude='lost+found'",
				"--slave-info",
				"--safe-slave-backup",
			},
		),
	)
})

var _ = Describe("MariadbOperatorBackup", func() {
	DescribeTable("builds operator backup args",
		func(backupCmd *BackupCommand, wantArgs []string) {
			command, err := backupCmd.MariadbOperatorBackup()
			Expect(err).NotTo(HaveOccurred())
			Expect(command.Args).To(Equal(wantArgs))
		},
		Entry("logical no S3 no cleanupTargetFile",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					BackupContentType:    mariadbv1alpha1.BackupContentTypeLogical,
					TargetFilePath:       "/backups/0-backup-target.txt",
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
				},
			},
			[]string{
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
		),
		Entry("physical no S3 no cleanupTargetFile",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					BackupContentType:    mariadbv1alpha1.BackupContentTypePhysical,
					MaxRetentionDuration: 24 * time.Hour,
					Compression:          mariadbv1alpha1.CompressGzip,
					LogLevel:             "info",
				},
			},
			[]string{
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
		),
		Entry("logical S3",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					BackupContentType:    mariadbv1alpha1.BackupContentTypeLogical,
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
			[]string{
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
		),
		Entry("physical S3",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					BackupContentType:    mariadbv1alpha1.BackupContentTypePhysical,
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
			[]string{
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
		),
		Entry("logical S3 and cleanup target file",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					BackupContentType:    mariadbv1alpha1.BackupContentTypeLogical,
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
			[]string{
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
		),
		Entry("physical S3 and cleanup target file",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:                 "/backups",
					TargetFilePath:       "/backups/0-backup-target.txt",
					BackupContentType:    mariadbv1alpha1.BackupContentTypePhysical,
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
			[]string{
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
		),
		Entry("physical S3 and meta",
			&BackupCommand{
				BackupOpts: BackupOpts{
					Path:               "/backups",
					TargetFilePath:     "/backups/0-backup-target.txt",
					BackupFullDirPath:  "/backups/full",
					BackupContentType:  mariadbv1alpha1.BackupContentTypePhysical,
					PhysicalBackupMeta: true,
					PhysicalBackupKey: &types.NamespacedName{
						Name:      "test",
						Namespace: "test",
					},
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
			[]string{
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
				"--physical-backup-dir-path",
				"/backups/full",
				"--physical-backup-meta",
				"--physical-backup-name",
				"test",
				"--physical-backup-namespace",
				"test",
			},
		),
	)
})

var _ = Describe("mariadbRestoreArgs", func() {
	DescribeTable("builds restore args",
		func(backupCmd *BackupCommand, restore *mariadbv1alpha1.Restore, mariadb *mariadbv1alpha1.MariaDB, wantArgs []string) {
			args := backupCmd.mariadbRestoreArgs(restore, mariadb)
			Expect(args).To(Equal(wantArgs))
		},
		Entry("empty",
			&BackupCommand{},
			&mariadbv1alpha1.Restore{},
			&mariadbv1alpha1.MariaDB{},
			nil,
		),
		Entry("args",
			&BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{
						"--verbose",
						"--one-database db1",
					},
				},
			},
			&mariadbv1alpha1.Restore{},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--verbose",
				"--one-database db1",
			},
		),
		Entry("duplicate args",
			&BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{
						"--verbose",
						"--verbose",
						"--one-database db1",
					},
				},
			},
			&mariadbv1alpha1.Restore{},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--verbose",
				"--one-database db1",
			},
		),
		Entry("database",
			&BackupCommand{},
			&mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					Database: "db1",
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--one-database db1",
			},
		),
		Entry("database and args",
			&BackupCommand{
				BackupOpts: BackupOpts{
					ExtraOpts: []string{
						"--verbose",
					},
				},
			},
			&mariadbv1alpha1.Restore{
				Spec: mariadbv1alpha1.RestoreSpec{
					Database: "db1",
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]string{
				"--verbose",
				"--one-database db1",
			},
		),
		Entry("TLS",
			&BackupCommand{},
			&mariadbv1alpha1.Restore{},
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			[]string{
				"--ssl",
				"--ssl-ca",
				builderpki.CACertPath,
				"--ssl-cert",
				builderpki.ClientCertPath,
				"--ssl-key",
				builderpki.ClientKeyPath,
				"--ssl-verify-server-cert",
			},
		),
	)
})

var _ = Describe("MariadbBackupRestore", func() {
	DescribeTable("builds restore command",
		func(backupCmd *BackupCommand, mariadb *mariadbv1alpha1.MariaDB, restoreOpts []MariaDBBackupRestoreOpt, wantErr, wantCleanup bool) {
			cmd, err := backupCmd.MariadbBackupRestore(
				mariadb,
				"/var/lib/mysql",
				restoreOpts...,
			)
			if wantErr {
				Expect(err).To(HaveOccurred())
				Expect(cmd).To(BeNil())
				return
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Args).NotTo(BeEmpty())
			script := cmd.Args[0] // NewBashCommand puts the whole script here
			if wantCleanup {
				Expect(script).To(ContainSubstring("rm -rf /var/lib/mysql/*"))
			} else {
				Expect(script).NotTo(ContainSubstring("rm -rf /var/lib/mysql/*"))
			}
		},
		Entry("with database option (should error)",
			&BackupCommand{
				BackupOpts: BackupOpts{
					CommandOpts: CommandOpts{
						Database: ptr.To("somedb"),
					},
					TargetFilePath:    "/backups/target.sql",
					BackupFullDirPath: "/backup/full",
				},
			},
			&mariadbv1alpha1.MariaDB{},
			nil,
			true,
			false,
		),
		Entry("basic physical restore",
			&BackupCommand{
				BackupOpts: BackupOpts{
					TargetFilePath:    "/backups/target.sql",
					BackupFullDirPath: "/backup/full",
				},
			},
			&mariadbv1alpha1.MariaDB{},
			nil,
			false,
			false,
		),
		Entry("with cleanup data dir (should include cleanup command)",
			&BackupCommand{
				BackupOpts: BackupOpts{
					TargetFilePath:    "/backups/target.sql",
					BackupFullDirPath: "/backup/full",
				},
			},
			&mariadbv1alpha1.MariaDB{},
			[]MariaDBBackupRestoreOpt{WithCleanupDataDir(true)},
			false,
			true,
		),
	)
})

var _ = Describe("MariadbOperatorPITR", func() {
	DescribeTable("builds PITR command",
		func(opts []BackupOpt, strictMode, wantErr bool, wantArgs []string) {
			allOpts := []BackupOpt{
				WithUserEnv("test"),
				WithPasswordEnv("test"),
			}
			allOpts = append(allOpts, opts...)
			b, err := NewBackupCommand(allOpts...)
			Expect(err).NotTo(HaveOccurred())

			cmd, err := b.MariadbOperatorPITR(strictMode)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

			Expect(cmd.Args).To(Equal(wantArgs))
		},
		Entry("basic PITR without S3",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
			},
			false,
			false,
			[]string{
				"pitr",
				"--path",
				"/binlogs",
				"--target-file-path",
				"/binlogs/file",
				"--start-gtid",
				"0-10-1",
				"--target-time",
				targetTime.Format(time.RFC3339),
			},
		),
		Entry("PITR with S3",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithS3("test-bucket", "s3.example.com", "us-west-2", "prefix/"),
				WithS3TLS(true),
				WithS3CACertPath("/ca/cert"),
			},
			false,
			false,
			[]string{
				"pitr",
				"--path",
				"/binlogs",
				"--target-file-path",
				"/binlogs/file",
				"--start-gtid",
				"0-10-1",
				"--target-time",
				targetTime.Format(time.RFC3339),
				"--s3",
				"--s3-bucket",
				"test-bucket",
				"--s3-endpoint",
				"s3.example.com",
				"--s3-region",
				"us-west-2",
				"--s3-tls",
				"--s3-ca-cert-path",
				"/ca/cert",
				"--s3-prefix",
				"prefix/",
			},
		),
		Entry("PITR with compression",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithCompression(mariadbv1alpha1.CompressGzip),
			},
			false,
			false,
			[]string{
				"pitr",
				"--path",
				"/binlogs",
				"--target-file-path",
				"/binlogs/file",
				"--start-gtid",
				"0-10-1",
				"--target-time",
				targetTime.Format(time.RFC3339),
				"--compression",
				"gzip",
			},
		),
		Entry("PITR with log level",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithLogLevel("debug"),
			},
			false,
			false,
			[]string{
				"pitr",
				"--path",
				"/binlogs",
				"--target-file-path",
				"/binlogs/file",
				"--start-gtid",
				"0-10-1",
				"--target-time",
				targetTime.Format(time.RFC3339),
				"--log-level",
				"debug",
			},
		),
		Entry("PITR with strict mode",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
			},
			true,
			false,
			[]string{
				"pitr",
				"--path",
				"/binlogs",
				"--target-file-path",
				"/binlogs/file",
				"--start-gtid",
				"0-10-1",
				"--target-time",
				targetTime.Format(time.RFC3339),
				"--strict-mode",
			},
		),
		Entry("PITR with all options",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithMaxRetention(30 * 24 * time.Hour),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithS3("test-bucket", "s3.example.com", "us-west-2", "prefix/"),
				WithS3TLS(true),
				WithS3CACertPath("/ca/cert"),
				WithCompression(mariadbv1alpha1.CompressBzip2),
				WithLogLevel("info"),
			},
			true,
			false,
			[]string{
				"pitr",
				"--path",
				"/binlogs",
				"--target-file-path",
				"/binlogs/file",
				"--start-gtid",
				"0-10-1",
				"--target-time",
				targetTime.Format(time.RFC3339),
				"--strict-mode",
				"--s3",
				"--s3-bucket",
				"test-bucket",
				"--s3-endpoint",
				"s3.example.com",
				"--s3-region",
				"us-west-2",
				"--s3-tls",
				"--s3-ca-cert-path",
				"/ca/cert",
				"--s3-prefix",
				"prefix/",
				"--compression",
				"bzip2",
				"--log-level",
				"info",
			},
		),
		Entry("PITR without startGtid",
			[]BackupOpt{
				WithPath("/backup", "/target/file", "/backup/full"),
				WithTargetTime(targetTime),
			},
			false,
			true,
			nil,
		),
	)
})

var _ = Describe("mariadbBinlogArgs", func() {
	DescribeTable("builds binlog restore args",
		func(opts []BackupOpt, mariadb *mariadbv1alpha1.MariaDB, wantArgs []string, wantErr bool) {
			b, err := NewBackupCommand(opts...)
			Expect(err).NotTo(HaveOccurred())

			args, err := b.mariadbBinlogArgs(mariadb)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(args).NotTo(BeNil())

			Expect(args).To(Equal(wantArgs))
		},
		Entry("error when StartGtid is nil",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithTargetTime(targetTime),
				WithUserEnv("test"),
				WithPasswordEnv("test"),
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Port: 3306,
				},
			},
			nil,
			true,
		),
		Entry("valid",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithUserEnv("test"),
				WithPasswordEnv("test"),
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Port: 3306,
				},
			},
			[]string{
				"set -euo pipefail",
				"echo 💾 Restoring binlogs",
				fmt.Sprintf(
					"TZ=UTC mariadb-binlog --start-position=\"%s\" --stop-datetime=\"%s\" $(cat '/binlogs/file') | mariadb %s",
					startGtid.String(),
					targetTime.UTC().Format(time.DateTime),
					mdbFlags,
				),
			},
			false,
		),
		Entry("valid with TLS",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithUserEnv("test"),
				WithPasswordEnv("test"),
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Port: 3306,
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			[]string{
				"set -euo pipefail",
				"echo 💾 Restoring binlogs",
				fmt.Sprintf(
					"TZ=UTC mariadb-binlog --start-position=\"%s\" --stop-datetime=\"%s\" $(cat '/binlogs/file') | mariadb %s %s",
					startGtid.String(),
					targetTime.UTC().Format(time.DateTime),
					mdbFlags,
					tlsFlags,
				),
			},
			false,
		),
		Entry("valid with extra args",
			[]BackupOpt{
				WithPath("/binlogs", "/binlogs/file", "/backup/full"),
				WithStartGtid(startGtid),
				WithTargetTime(targetTime),
				WithUserEnv("test"),
				WithPasswordEnv("test"),
				WithExtraOpts([]string{
					"--log-level=debug",
				}),
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: mdbObjectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
					Port: 3306,
				},
			},
			[]string{
				"set -euo pipefail",
				"echo 💾 Restoring binlogs",
				fmt.Sprintf(
					"TZ=UTC mariadb-binlog --start-position=\"%s\" --stop-datetime=\"%s\" $(cat '/binlogs/file') | mariadb %s --log-level=debug",
					startGtid.String(),
					targetTime.UTC().Format(time.DateTime),
					mdbFlags,
				),
			},
			false,
		),
	)
})

var _ = Describe("physicalBackupArgs", func() {
	DescribeTable("builds physical backup args",
		func(backupContentType mariadbv1alpha1.BackupContentType, backupFullDirPath string,
			physicalBackupMeta bool, physicalBackupKey *types.NamespacedName, wantArgs []string) {
			b := &BackupCommand{
				BackupOpts: BackupOpts{
					BackupContentType:  backupContentType,
					BackupFullDirPath:  backupFullDirPath,
					PhysicalBackupMeta: physicalBackupMeta,
					PhysicalBackupKey:  physicalBackupKey,
				},
			}

			Expect(b.physicalBackupArgs()).To(Equal(wantArgs))
		},
		Entry("Non-physical backup content type",
			mariadbv1alpha1.BackupContentTypeLogical,
			"/backup/dir",
			false,
			(*types.NamespacedName)(nil),
			nil,
		),
		Entry("Physical backup with directory path",
			mariadbv1alpha1.BackupContentTypePhysical,
			"/backup/dir",
			false,
			(*types.NamespacedName)(nil),
			[]string{
				"--physical-backup-dir-path",
				"/backup/dir",
			},
		),
		Entry("Physical backup with meta and key",
			mariadbv1alpha1.BackupContentTypePhysical,
			"/backup/dir",
			true,
			&types.NamespacedName{
				Name:      "test-backup",
				Namespace: "test-namespace",
			},
			[]string{
				"--physical-backup-dir-path",
				"/backup/dir",
				"--physical-backup-meta",
				"--physical-backup-name",
				"test-backup",
				"--physical-backup-namespace",
				"test-namespace",
			},
		),
		Entry("Physical backup with directory and meta but no key",
			mariadbv1alpha1.BackupContentTypePhysical,
			"/backup/dir",
			true,
			(*types.NamespacedName)(nil),
			[]string{
				"--physical-backup-dir-path",
				"/backup/dir",
			},
		),
	)
})

func mustParseGtid(rawGtid string) *replication.Gtid {
	gtid, err := replication.ParseGtid(rawGtid)
	if err != nil {
		panic(fmt.Sprintf("unexpected error parsing GTID: %v", err))
	}
	return gtid
}
