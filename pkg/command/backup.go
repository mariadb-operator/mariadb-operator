package command

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	backuppkg "github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	ds "github.com/mariadb-operator/mariadb-operator/v25/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"k8s.io/utils/ptr"
)

type BackupOpts struct {
	CommandOpts
	Path                 string
	TargetFilePath       string
	OmitCredentials      bool
	CleanupTargetFile    bool
	MaxRetentionDuration time.Duration
	TargetTime           time.Time
	Compression          mariadbv1alpha1.CompressAlgorithm
	S3                   bool
	S3Bucket             string
	S3Endpoint           string
	S3Region             string
	S3TLS                bool
	S3CACertPath         string
	S3Prefix             string
	LogLevel             string
	ExtraOpts            []string
}

type BackupOpt func(*BackupOpts)

func WithBackup(path string, targetFilePath string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Path = path
		bo.TargetFilePath = targetFilePath
	}
}

func WithOmitCredentials(omit bool) BackupOpt {
	return func(bo *BackupOpts) {
		bo.OmitCredentials = omit
	}
}

func WithCleanupTargetFile(shouldCleanup bool) BackupOpt {
	return func(bo *BackupOpts) {
		bo.CleanupTargetFile = shouldCleanup
	}
}

func WithBackupMaxRetention(d time.Duration) BackupOpt {
	return func(bo *BackupOpts) {
		bo.MaxRetentionDuration = d
	}
}

func WithBackupTargetTime(t time.Time) BackupOpt {
	return func(bo *BackupOpts) {
		bo.TargetTime = t
	}
}

func WithBackupCompression(c mariadbv1alpha1.CompressAlgorithm) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Compression = c
	}
}

func WithS3(bucket, endpoint, region, prefix string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.S3 = true
		bo.S3Bucket = bucket
		bo.S3Endpoint = endpoint
		bo.S3Region = region
		bo.S3Prefix = prefix
	}
}

func WithS3TLS(tls bool) BackupOpt {
	return func(bo *BackupOpts) {
		bo.S3TLS = tls
	}
}

func WithS3CACertPath(caCertPath string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.S3CACertPath = caCertPath
	}
}

func WithExtraOpts(opts []string) BackupOpt {
	return func(o *BackupOpts) {
		o.ExtraOpts = opts
	}
}

func WithBackupUserEnv(u string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.UserEnv = u
	}
}

func WithBackupPasswordEnv(p string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.PasswordEnv = p
	}
}

func WithBackupDatabase(d string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Database = &d
	}
}

func WithBackupLogLevel(l string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.LogLevel = l
	}
}

type BackupCommand struct {
	BackupOpts
}

func NewBackupCommand(userOpts ...BackupOpt) (*BackupCommand, error) {
	opts := BackupOpts{}
	for _, setOpt := range userOpts {
		setOpt(&opts)
	}
	if opts.Path == "" {
		return nil, errors.New("path not provided")
	}
	if opts.TargetFilePath == "" {
		return nil, errors.New("target file not provided")
	}
	if opts.MaxRetentionDuration == 0 {
		opts.MaxRetentionDuration = 30 * 24 * time.Hour
	}
	if opts.TargetTime.Equal(time.Time{}) {
		opts.TargetTime = time.Now()
	}
	if !opts.OmitCredentials {
		if opts.UserEnv == "" {
			return nil, errors.New("user environment variable not provided")
		}
		if opts.PasswordEnv == "" {
			return nil, errors.New("password environment variable not provided")
		}
	}
	return &BackupCommand{opts}, nil
}

func (b *BackupCommand) MariadbDump(backup *mariadbv1alpha1.Backup,
	mariadb *mariadbv1alpha1.MariaDB) (*Command, error) {
	connFlags, err := ConnectionFlags(&b.CommandOpts, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting connection flags: %v", err)
	}
	args := strings.Join(b.mariadbDumpArgs(backup, mariadb), " ")

	cmds := []string{
		"set -euo pipefail",
		"echo 💾 Exporting env",
		fmt.Sprintf(
			"export BACKUP_FILE=%s",
			b.newBackupFile(),
		),
		fmt.Sprintf(
			"echo 💾 Writing target file: %s",
			b.TargetFilePath,
		),
		fmt.Sprintf(
			"printf \"${BACKUP_FILE}\" > %s",
			b.TargetFilePath,
		),
		fmt.Sprintf(
			"echo 💾 Taking backup: %s",
			b.getTargetFilePath(),
		),
		fmt.Sprintf(
			"mariadb-dump %s %s > %s",
			connFlags,
			args,
			b.getTargetFilePath(),
		),
	}
	return NewBashCommand(cmds), nil
}

func (b *BackupCommand) MariadbBackup(mariadb *mariadbv1alpha1.MariaDB, backupFilePath string,
	targetPodIndex int) (*Command, error) {
	if b.Database != nil {
		return nil, errors.New("database option not supported in physical backups")
	}

	host := statefulset.PodFQDNWithService(mariadb.ObjectMeta, targetPodIndex, mariadb.InternalServiceKey().Name)
	connFlags, err := ConnectionFlags(
		&b.CommandOpts,
		mariadb,
		WithHostConnectionFlag(host),
	)
	if err != nil {
		return nil, fmt.Errorf("error getting connection flags: %v", err)
	}
	args := strings.Join(b.mariadbBackupArgs(mariadb), " ")

	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(
			"echo 💾 Writing target file: %s",
			b.TargetFilePath,
		),
		fmt.Sprintf(
			"printf \"%s\" > %s",
			backupFilePath,
			b.TargetFilePath,
		),
		fmt.Sprintf(
			"echo 💾 Taking backup: %s",
			b.getTargetFilePath(),
		),
		fmt.Sprintf(
			"mariadb-backup %s %s > %s",
			connFlags,
			args,
			b.getTargetFilePath(),
		),
	}
	return NewBashCommand(cmds), nil
}

func (b *BackupCommand) MariadbOperatorBackup(backupContentType mariadbv1alpha1.BackupContentType) *Command {
	args := []string{
		"backup",
		"--path",
		b.Path,
		"--target-file-path",
		b.TargetFilePath,
		"--backup-content-type",
		string(backupContentType),
		"--max-retention",
		b.MaxRetentionDuration.String(),
	}
	if b.Compression != "" {
		args = append(args, []string{
			"--compression",
			string(b.Compression),
		}...)
	}
	if b.LogLevel != "" {
		args = append(args, []string{
			"--log-level",
			b.LogLevel,
		}...)
	}

	args = append(args, b.s3Args()...)
	if b.S3 && b.CleanupTargetFile {
		args = append(args, "--cleanup-target-file")
	}

	return NewCommand(nil, args)
}

func (b *BackupCommand) MariadbOperatorRestore(backupContentType mariadbv1alpha1.BackupContentType, backupDirPath *string) *Command {
	args := []string{
		"backup",
		"restore",
		"--path",
		b.Path,
		"--target-time",
		backuppkg.FormatBackupDate(b.TargetTime),
		"--target-file-path",
		b.TargetFilePath,
		"--backup-content-type",
		string(backupContentType),
	}
	if backupContentType == mariadbv1alpha1.BackupContentTypePhysical && backupDirPath != nil {
		args = append(args, []string{
			"--physical-backup-dir-path",
			*backupDirPath,
		}...)
	}
	if b.LogLevel != "" {
		args = append(args, []string{
			"--log-level",
			b.LogLevel,
		}...)
	}

	args = append(args, b.s3Args()...)
	return NewCommand(nil, args)
}

func (b *BackupCommand) MariadbRestore(restore *mariadbv1alpha1.Restore, mariadb *mariadbv1alpha1.MariaDB) (*Command, error) {
	connFlags, err := ConnectionFlags(&b.CommandOpts, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting connection flags: %v", err)
	}
	args := strings.Join(b.mariadbArgs(restore, mariadb), " ")
	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(
			"echo 💾 Restoring backup: %s",
			b.getTargetFilePath(),
		),
		fmt.Sprintf(
			"mariadb %s %s < %s",
			connFlags,
			args,
			b.getTargetFilePath(),
		),
	}
	return NewBashCommand(cmds), nil
}

func (b *BackupCommand) MariadbBackupRestore(mariadb *mariadbv1alpha1.MariaDB, backupDirPath string) (*Command, error) {
	if b.Database != nil {
		return nil, errors.New("database option not supported in physical backups")
	}

	// The ext4 filesystem creates a lost+found directory by default, which causes mariadb-backup to fail with:
	// "Original data directory /var/lib/mysql is not empty!"
	// Since we already check the PVC existence earlier, it should be safe to use --force-non-empty-directories.
	copyBackupCmd := fmt.Sprintf(
		"mariadb-backup --copy-back --target-dir=%s --force-non-empty-directories",
		backupDirPath,
	)

	cmds := []string{
		"set -euo pipefail",
		"echo 💾 Checking existing backup",
		fmt.Sprintf(
			"if [ -d %s ]; then echo '💾 Existing backup directory found. Copying backup to data directory'; %s && exit 0; fi",
			backupDirPath,
			copyBackupCmd,
		),
		"echo 💾 Extracting backup",
		fmt.Sprintf(
			"mkdir %s",
			backupDirPath,
		),
		fmt.Sprintf(
			"mbstream -x -C %s < %s",
			backupDirPath,
			b.getTargetFilePath(),
		),
		"echo 💾 Preparing backup",
		fmt.Sprintf(
			"mariadb-backup --prepare --target-dir=%s",
			backupDirPath,
		),
		"echo 💾 Copying backup to data directory",
		copyBackupCmd,
	}
	return NewBashCommand(cmds), nil
}

func (b *BackupCommand) newBackupFile() string {
	var fileName string
	if b.Compression == mariadbv1alpha1.CompressNone {
		fileName = fmt.Sprintf(
			"backup.$(date -u +'%s').sql",
			"%Y-%m-%dT%H:%M:%SZ",
		)
	} else {
		fileName = fmt.Sprintf(
			"backup.$(date -u +'%s').%v.sql",
			"%Y-%m-%dT%H:%M:%SZ",
			b.Compression,
		)
	}
	return filepath.Join(b.Path, fileName)
}

func (b *BackupCommand) getTargetFilePath() string {
	return fmt.Sprintf("$(cat '%s')", b.TargetFilePath)
}

func (b *BackupCommand) mariadbDumpArgs(backup *mariadbv1alpha1.Backup, mariadb *mariadbv1alpha1.MariaDB) []string {
	dumpOpts := make([]string, len(b.ExtraOpts))
	copy(dumpOpts, b.ExtraOpts)

	args := []string{
		"--single-transaction",
		"--events",
		"--routines",
	}

	hasDatabasesOpt := func(do string) bool {
		return strings.HasPrefix(strings.TrimSpace(do), "--databases")
	}
	hasDatabases := ds.Any(dumpOpts, hasDatabasesOpt)

	if len(backup.Spec.Databases) > 0 {
		args = append(args, fmt.Sprintf("--databases %s", strings.Join(backup.Spec.Databases, " ")))
		if hasDatabases {
			dumpOpts = ds.Remove(dumpOpts, hasDatabasesOpt)
		}
	} else if !hasDatabases {
		args = append(args, "--all-databases")
	}

	// LOCK TABLES is not compatible with Galera: https://mariadb.com/kb/en/lock-tables/#limitations
	if mariadb.IsGaleraEnabled() {
		args = append(args, "--skip-add-locks")
	}
	// Galera only replicates InnoDB tables and mysql.global_priv uses the MyISAM engine.
	// Ignoring this table enables a clean restore without replicas getting restarted
	// because the livenessProbe fails due to authentication errors.
	// Users and grants should be created by the entrypoint or the User and Grant CRs.
	// See: https://github.com/mariadb-operator/mariadb-operator/issues/556
	if ptr.Deref(backup.Spec.IgnoreGlobalPriv, false) {
		args = append(args, "--ignore-table=mysql.global_priv")
	}

	if mariadb.IsTLSEnabled() {
		args = append(args, b.tlsArgs(mariadb)...)
	}

	return ds.Unique(ds.Merge(args, dumpOpts)...)
}

func (b *BackupCommand) mariadbBackupArgs(mariadb *mariadbv1alpha1.MariaDB) []string {
	backupOpts := make([]string, len(b.ExtraOpts))
	copy(backupOpts, b.ExtraOpts)

	args := []string{
		"--backup",
		"--stream=xbstream",
		// The ext4 filesystem creates a lost+found directory by default,
		// which causes mariadb-backup to include it in the backup file as a database.
		"--databases-exclude='lost+found'",
	}

	if mariadb.IsTLSEnabled() {
		args = append(args, b.tlsArgs(mariadb)...)
	}

	return ds.Unique(ds.Merge(args, backupOpts)...)
}

func (b *BackupCommand) mariadbArgs(restore *mariadbv1alpha1.Restore, mariadb *mariadbv1alpha1.MariaDB) []string {
	args := make([]string, len(b.ExtraOpts))
	copy(args, b.ExtraOpts)

	if restore.Spec.Database != "" {
		args = append(args, fmt.Sprintf("--one-database %s", restore.Spec.Database))
	}

	if mariadb.IsTLSEnabled() {
		args = append(args, b.tlsArgs(mariadb)...)
	}

	return ds.Unique(args...)
}

func (b *BackupCommand) s3Args() []string {
	if !b.S3 {
		return nil
	}
	args := []string{
		"--s3",
		"--s3-bucket",
		b.S3Bucket,
		"--s3-endpoint",
		b.S3Endpoint,
	}
	if b.S3Region != "" {
		args = append(args,
			"--s3-region",
			b.S3Region,
		)
	}
	if b.S3TLS {
		args = append(args,
			"--s3-tls",
		)
		if b.S3CACertPath != "" {
			args = append(args,
				"--s3-ca-cert-path",
				b.S3CACertPath,
			)
		}
	}
	if b.S3Prefix != "" {
		args = append(args,
			"--s3-prefix",
			b.S3Prefix,
		)
	}
	return args
}

func (b *BackupCommand) tlsArgs(mariadb *mariadbv1alpha1.MariaDB) []string {
	if !mariadb.IsTLSEnabled() {
		return nil
	}
	return []string{
		"--ssl",
		"--ssl-ca",
		builderpki.CACertPath,
		"--ssl-cert",
		builderpki.ClientCertPath,
		"--ssl-key",
		builderpki.ClientKeyPath,
		"--ssl-verify-server-cert",
	}
}
