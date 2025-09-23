package command

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	backuppkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	builderpki "github.com/mariadb-operator/mariadb-operator/v26/pkg/builder/pki"
	ds "github.com/mariadb-operator/mariadb-operator/v26/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/interfaces"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

type BackupOpts struct {
	CommandOpts
	Path                   string
	TargetFilePath         string
	BackupFullDirPath      string
	BackupContentType      mariadbv1alpha1.BackupContentType
	PhysicalBackupMeta     bool
	PhysicalBackupKey      *types.NamespacedName
	OmitCredentials        bool
	CleanupTargetFile      bool
	MaxRetentionDuration   time.Duration
	StartGtid              *replication.Gtid
	TargetTime             time.Time
	TargetTimeAgeThreshold *time.Time
	Compression            mariadbv1alpha1.CompressAlgorithm
	LogLevel               string
	ExtraOpts              []string

	S3           bool
	S3Bucket     string
	S3Endpoint   string
	S3Region     string
	S3TLS        bool
	S3CACertPath string
	S3Prefix     string

	ABS              bool
	ABSContainerName string
	ABSServiceURL    string
	ABSTLS           bool
	ABSCACertPath    string
	ABSPrefix        string
}

type BackupOpt func(*BackupOpts)

func WithPath(path, targetFilePath, backupFullDirPath string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Path = path
		bo.TargetFilePath = targetFilePath
		bo.BackupFullDirPath = backupFullDirPath
	}
}

func WithBackupContentType(backupContentType mariadbv1alpha1.BackupContentType) BackupOpt {
	return func(bo *BackupOpts) {
		bo.BackupContentType = backupContentType
	}
}

func WithPhysicalBackupMeta(enabled bool, physicalBackupKey types.NamespacedName) BackupOpt {
	return func(bo *BackupOpts) {
		bo.PhysicalBackupMeta = enabled
		bo.PhysicalBackupKey = &physicalBackupKey
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

func WithMaxRetention(d time.Duration) BackupOpt {
	return func(bo *BackupOpts) {
		bo.MaxRetentionDuration = d
	}
}

func WithStartGtid(gtid *replication.Gtid) BackupOpt {
	return func(bo *BackupOpts) {
		bo.StartGtid = gtid
	}
}

func WithTargetTime(t time.Time) BackupOpt {
	return func(bo *BackupOpts) {
		bo.TargetTime = t
	}
}

func WithCompression(c mariadbv1alpha1.CompressAlgorithm) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Compression = c
	}
}

func WithBackupTargetTimeAgeThreshold(threshold *time.Time) BackupOpt {
	return func(bo *BackupOpts) {
		bo.TargetTimeAgeThreshold = threshold
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

func WithABS(containerName, serviceURL, prefix string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.ABS = true
		bo.ABSContainerName = containerName
		bo.ABSServiceURL = serviceURL
		bo.ABSPrefix = prefix
	}
}

func WithABSTLS(tls bool) BackupOpt {
	return func(bo *BackupOpts) {
		bo.ABSTLS = tls
	}
}

func WithABSCACertPath(caCertPath string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.ABSCACertPath = caCertPath
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

func WithUserEnv(u string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.UserEnv = u
	}
}

func WithPasswordEnv(p string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.PasswordEnv = p
	}
}

func WithDatabase(d string) BackupOpt {
	return func(bo *BackupOpts) {
		bo.Database = &d
	}
}

func WithLogLevel(l string) BackupOpt {
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
	if opts.BackupFullDirPath == "" {
		return nil, errors.New("backup full directory not provided")
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
	mariadb interfaces.MariaDBObject) (*Command, error) {
	connFlags, err := ConnectionFlags(&b.CommandOpts, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting connection flags: %v", err)
	}
	args := strings.Join(b.mariadbDumpArgs(backup, mariadb), " ")
	tablesBySchema := groupTablesBySchema(backup.Spec.Tables)
	isMultiSchema := len(tablesBySchema) > 1

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
	}

	dumpArgs := args
	if isMultiSchema {
		// mapfile reads each row as a separate array element so identifiers with spaces
		// (e.g. `lerg`.`LERG 6 ATC`) survive intact; "${MARIADB_IGNORE_ARGS[@]}" expands
		// each element as one shell word rather than word-splitting on whitespace.
		cmds = append(cmds,
			"echo 💾 Building ignore-table flags",
			fmt.Sprintf(
				`mapfile -t MARIADB_IGNORE_ARGS < <(mariadb %s -BNe "%s")`,
				connFlags,
				buildIgnoreTableQuery(tablesBySchema),
			),
		)
		dumpArgs = args + ` "${MARIADB_IGNORE_ARGS[@]}"`
	}

	cmds = append(cmds,
		fmt.Sprintf(
			"echo 💾 Taking backup: %s",
			b.getTargetFilePath(),
		),
		fmt.Sprintf(
			"mariadb-dump %s %s > %s",
			connFlags,
			dumpArgs,
			b.getTargetFilePath(),
		),
	)
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
	args := strings.Join(b.mariadbBackupArgs(mariadb, targetPodIndex), " ")

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

func (b *BackupCommand) MariadbBackupMeta() *Command {
	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(`if [ -d %[1]s ]; then
	echo "💾 Cleaning up backup directory";
	rm -rf %[1]s
fi`, b.BackupFullDirPath),
		"echo 💾 Creating backup directory",
		fmt.Sprintf(
			"mkdir -p %s",
			b.BackupFullDirPath,
		),
		"echo 💾 Extracting stream",
		fmt.Sprintf(
			"mbstream -x -C %s < %s",
			b.BackupFullDirPath,
			b.getTargetFilePath(),
		),
	}
	cmds = append(cmds, copyBinlogMetaCmds(b.BackupFullDirPath, b.BackupFullDirPath)...)
	return NewBashCommand(cmds)
}

func (b *BackupCommand) MariadbOperatorBackup() (*Command, error) {
	if b.BackupContentType == "" {
		return nil, errors.New("backup content type must be set")
	}
	args := []string{
		"backup",
		"--path",
		b.Path,
		"--target-file-path",
		b.TargetFilePath,
		"--backup-content-type",
		string(b.BackupContentType),
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
	args = append(args, b.absArgs()...)
	if (b.S3 || b.ABS) && b.CleanupTargetFile {
		args = append(args, "--cleanup-target-file")
	}
	args = append(args, b.physicalBackupArgs()...)

	return NewCommand(nil, args), nil
}

func (b *BackupCommand) MariadbOperatorRestore() (*Command, error) {
	if b.BackupContentType == "" {
		return nil, errors.New("backup content type must be set")
	}
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
		string(b.BackupContentType),
	}

	if b.TargetTimeAgeThreshold != nil {
		args = append(args, []string{
			"--target-time-age-threshold",
			backuppkg.FormatBackupDate(*b.TargetTimeAgeThreshold),
		}...)
	}

	if b.LogLevel != "" {
		args = append(args, []string{
			"--log-level",
			b.LogLevel,
		}...)
	}

	args = append(args, b.s3Args()...)
	args = append(args, b.absArgs()...)
	args = append(args, b.physicalBackupArgs()...)

	return NewCommand(nil, args), nil
}

func (b *BackupCommand) MariadbRestore(restore *mariadbv1alpha1.Restore,
	mariadb interfaces.MariaDBObject) (*Command, error) {

	var err error
	var connFlags string
	if restore.Spec.PodIndex != nil {
		connFlags, err = PodConnectionFlags(&b.CommandOpts, mariadb, *restore.Spec.PodIndex)
	} else {
		connFlags, err = ConnectionFlags(&b.CommandOpts, mariadb)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting connection flags: %v", err)
	}

	args := strings.Join(b.mariadbRestoreArgs(restore, mariadb), " ")
	cmds := []string{
		"set -euo pipefail",
		fmt.Sprintf(
			"echo 💾 Restoring backup: %s",
			b.getTargetFilePath(),
		),
	}
	if restore.Spec.Database != "" {
		cmds = append(cmds, fmt.Sprintf(
			"mariadb %s -e 'CREATE DATABASE IF NOT EXISTS `%s`;'",
			connFlags,
			restore.Spec.Database,
		))
	}
	cmds = append(cmds, fmt.Sprintf(
		"mariadb %s %s < %s",
		connFlags,
		args,
		b.getTargetFilePath(),
	))
	return NewBashCommand(cmds), nil
}

type MariaDBBackupRestoreOpts struct {
	cleanupDataDir bool
}

type MariaDBBackupRestoreOpt func(*MariaDBBackupRestoreOpts)

func WithCleanupDataDir(cleanup bool) MariaDBBackupRestoreOpt {
	return func(mdro *MariaDBBackupRestoreOpts) {
		mdro.cleanupDataDir = cleanup
	}
}

func (b *BackupCommand) MariadbBackupRestore(mariadb *mariadbv1alpha1.MariaDB, dataDirPath string,
	restoreOpts ...MariaDBBackupRestoreOpt) (*Command, error) {
	if b.Database != nil {
		return nil, errors.New("database option not supported in physical backups")
	}
	opts := MariaDBBackupRestoreOpts{}
	for _, setOpt := range restoreOpts {
		setOpt(&opts)
	}

	// Replicas being recovered will have a data directory in error state, needs to be cleaned up before restoring.
	cleanupDataDirCmd := `if [ -d /var/lib/mysql ]; then 
	echo "💾 Cleaning up data directory";
	rm -rf /var/lib/mysql/*;
fi`
	// The ext4 filesystem creates a lost+found directory by default, which causes mariadb-backup to fail with:
	// "Original data directory /var/lib/mysql is not empty!"
	// Since we already check the PVC existence earlier, it should be safe to use --force-non-empty-directories.
	copyBackupCmd := fmt.Sprintf(
		"mariadb-backup --copy-back --target-dir=%s --force-non-empty-directories",
		b.BackupFullDirPath,
	)

	cmds := []string{
		"set -euo pipefail",
		"echo 💾 Extracting backup",
		fmt.Sprintf(
			"mkdir -p %s",
			b.BackupFullDirPath,
		),
		fmt.Sprintf(
			"mbstream -x -C %s < %s",
			b.BackupFullDirPath,
			b.getTargetFilePath(),
		),
		"echo 💾 Preparing backup",
		fmt.Sprintf(
			"mariadb-backup --prepare --target-dir=%s",
			b.BackupFullDirPath,
		),
	}
	if opts.cleanupDataDir {
		cmds = append(cmds, cleanupDataDirCmd)
	}
	cmds = append(cmds, []string{
		"echo 💾 Copying backup to data directory",
		copyBackupCmd,
	}...)
	cmds = append(cmds, copyBinlogMetaCmds(b.BackupFullDirPath, dataDirPath)...)
	return NewBashCommand(cmds), nil
}

func (b *BackupCommand) MariadbOperatorPITR(strictMode bool) (*Command, error) {
	if b.StartGtid == nil {
		return nil, errors.New("startGtid must be set")
	}
	args := []string{
		"pitr",
		"--path",
		b.Path,
		"--target-file-path",
		b.TargetFilePath,
		"--start-gtid",
		b.StartGtid.String(),
		"--target-time",
		b.TargetTime.Format(time.RFC3339),
	}
	if strictMode {
		args = append(args, "--strict-mode")
	}
	args = append(args, b.s3Args()...)
	args = append(args, b.absArgs()...)

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

	return NewCommand(nil, args), nil
}

func (b *BackupCommand) MariadbBinlog(mariadb *mariadbv1alpha1.MariaDB) (*Command, error) {
	mariadbBinlogArgs, err := b.mariadbBinlogArgs(mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting mariadb-binlog args: %v", err)
	}
	return NewBashCommand(mariadbBinlogArgs), nil
}

func (b *BackupCommand) newBackupFile() string {
	var fileName string
	if b.Compression == mariadbv1alpha1.CompressNone {
		fileName = fmt.Sprintf(
			"backup.$(date -u +'%s').sql",
			"%Y-%m-%dT%H:%M:%SZ",
		)
	} else {
		// Use standard extension format: .sql.gz or .sql.bz2
		// This allows tools like gunzip to recognize the file format
		ext, _ := b.Compression.Extension()
		fileName = fmt.Sprintf(
			"backup.$(date -u +'%s').sql.%s",
			"%Y-%m-%dT%H:%M:%SZ",
			ext,
		)
	}
	return filepath.Join(b.Path, fileName)
}

func (b *BackupCommand) getTargetFilePath() string {
	return fmt.Sprintf("$(cat '%s')", b.TargetFilePath)
}

func (b *BackupCommand) mariadbDumpArgs(backup *mariadbv1alpha1.Backup, mariadb interfaces.MariaDBObject) []string {
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
	} else if len(backup.Spec.Tables) > 0 {
		tablesBySchema := groupTablesBySchema(backup.Spec.Tables)
		if len(tablesBySchema) > 1 {
			// Multi-schema: list all target databases; per-table filtering is applied at
			// runtime via --ignore-table flags built by querying information_schema.
			schemas := make([]string, 0, len(tablesBySchema))
			for s := range tablesBySchema {
				schemas = append(schemas, s)
			}
			sort.Strings(schemas)
			args = append(args, "--databases")
			args = append(args, schemas...)
		} else {
			// Single schema: --databases db --tables tbl1 tbl2 includes CREATE DATABASE / USE
			// statements (the plain positional form does not), required for a clean restore.
			db, _ := tableSelectionArgs(backup.Spec.Tables)
			args = append(args, "--databases", db)
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

	result := ds.UniqueArgs(ds.Merge(args, dumpOpts)...)

	// --tables must come after all other flags; it overrides --databases to limit
	// which tables are dumped while still emitting the database context statements.
	// Not used for multi-schema: ignore-table flags are injected at runtime instead.
	if len(backup.Spec.Tables) > 0 && len(groupTablesBySchema(backup.Spec.Tables)) == 1 {
		_, tables := tableSelectionArgs(backup.Spec.Tables)
		result = append(result, "--tables")
		result = append(result, tables...)
	}

	return result
}

// groupTablesBySchema groups "db.table" entries into a map of schema → []table.
func groupTablesBySchema(tables []string) map[string][]string {
	result := make(map[string][]string)
	for _, t := range tables {
		schema, table, found := strings.Cut(t, ".")
		if !found {
			continue
		}
		result[schema] = append(result[schema], table)
	}
	return result
}

// buildIgnoreTableQuery returns a SQL query that emits one "--ignore-table=schema.table"
// token per row for every BASE TABLE or VIEW in the given schemas that is NOT in
// tablesBySchema. Views are included so the dump doesn't try to recreate views that
// reference tables excluded from the filtered backup.
func buildIgnoreTableQuery(tablesBySchema map[string][]string) string {
	schemas := make([]string, 0, len(tablesBySchema))
	for s := range tablesBySchema {
		schemas = append(schemas, s)
	}
	sort.Strings(schemas)

	quotedSchemas := make([]string, len(schemas))
	for i, s := range schemas {
		quotedSchemas[i] = "'" + s + "'"
	}

	var pairs []string
	for _, s := range schemas {
		for _, t := range tablesBySchema[s] {
			pairs = append(pairs, fmt.Sprintf("('%s','%s')", s, t))
		}
	}

	return fmt.Sprintf(
		"SELECT CONCAT('--ignore-table=', TABLE_SCHEMA, '.', TABLE_NAME)"+
			" FROM information_schema.TABLES"+
			" WHERE TABLE_SCHEMA IN (%s)"+
			" AND TABLE_TYPE IN ('BASE TABLE','VIEW')"+
			" AND (TABLE_SCHEMA, TABLE_NAME) NOT IN (%s)",
		strings.Join(quotedSchemas, ","),
		strings.Join(pairs, ","),
	)
}

// tableSelectionArgs parses "db.table" entries into a database name and table list.
// All entries are expected to share the same database.
func tableSelectionArgs(tables []string) (string, []string) {
	var db string
	var tableNames []string
	for _, t := range tables {
		d, tbl, found := strings.Cut(t, ".")
		if !found {
			continue
		}
		if db == "" {
			db = d
		}
		tableNames = append(tableNames, tbl)
	}
	return db, tableNames
}

func (b *BackupCommand) mariadbBinlogArgs(mariadb *mariadbv1alpha1.MariaDB) ([]string, error) {
	if b.StartGtid == nil {
		return nil, errors.New("startGtid must be set")
	}
	connFlags, err := ConnectionFlags(&b.CommandOpts, mariadb)
	if err != nil {
		return nil, fmt.Errorf("error getting connection flags: %v", err)
	}
	mariadbArgs := b.mariadbArgs(mariadb)

	mariadbCmd := fmt.Sprintf("mariadb %s", connFlags)
	if len(mariadbArgs) > 0 {
		mariadbCmd += fmt.Sprintf(" %s", strings.Join(mariadbArgs, " "))
	}

	return []string{
		"set -euo pipefail",
		"echo 💾 Restoring binlogs",
		// TODO: pass multiple --start-position
		// See:
		// https://mariadb.com/docs/server/clients-and-utilities/logging-tools/mariadb-binlog/mariadb-binlog-options#j-pos-start-position-pos
		// https://jira.mariadb.org/browse/MDEV-37231
		// Note:
		// mariadb-binlog assumes the same timezone as the OS where it runs.
		// Here we enforce UTC and use a format compatible with the server.
		// The server can be in any timezone, mariadb-binlog handles that.
		fmt.Sprintf(
			"TZ=UTC mariadb-binlog --start-position=\"%s\" --stop-datetime=\"%s\" %s | %s",
			b.StartGtid.String(),
			b.TargetTime.UTC().Format(time.DateTime),
			b.getTargetFilePath(),
			mariadbCmd,
		),
	}, nil
}

func (b *BackupCommand) mariadbBackupArgs(mariadb *mariadbv1alpha1.MariaDB, targetPodIndex int) []string {
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
	if mariadb.IsReplicationEnabled() &&
		mariadb.Status.CurrentPrimaryPodIndex != nil && *mariadb.Status.CurrentPrimaryPodIndex != targetPodIndex {
		args = append(args, []string{
			"--slave-info",
			"--safe-slave-backup",
		}...)
	}

	return ds.UniqueArgs(ds.Merge(args, backupOpts)...)
}

func (b *BackupCommand) mariadbRestoreArgs(restore *mariadbv1alpha1.Restore, mariadb interfaces.MariaDBObject) []string {
	args := b.mariadbArgs(mariadb)

	if restore.Spec.Database != "" {
		repl := mariadb.Replication()
		isFilteredReplication := repl.ReplicaFromExternal != nil &&
			len(repl.ReplicaFromExternal.FilteredReplicaTables) > 0
		if isFilteredReplication {
			// Filtered-table dumps have no USE statements; --database sets the connection
			// default database upfront so every statement runs in the right context.
			args = append(args, fmt.Sprintf("--database %s", restore.Spec.Database))
		} else {
			args = append(args, fmt.Sprintf("--one-database %s", restore.Spec.Database))
		}
	}

	return ds.UniqueArgs(args...)
}

func (b *BackupCommand) mariadbArgs(mariadb interfaces.TLSProvider) []string {
	args := make([]string, len(b.ExtraOpts))
	copy(args, b.ExtraOpts)

	if mariadb.IsTLSEnabled() {
		args = append(args, b.tlsArgs(mariadb)...)
	}

	return ds.UniqueArgs(args...)
}

func (b *BackupCommand) absArgs() []string {
	if !b.ABS {
		return nil
	}
	args := []string{
		"--abs",
		"--abs-container",
		b.ABSContainerName,
		"--abs-service-url",
		b.ABSServiceURL,
	}
	if b.ABSTLS {
		args = append(args,
			"--abs-tls",
		)
		if b.ABSCACertPath != "" {
			args = append(args,
				"--abs-ca-cert-path",
				b.ABSCACertPath,
			)
		}
	}
	if b.ABSPrefix != "" {
		args = append(args,
			"--abs-prefix",
			b.ABSPrefix,
		)
	}
	return args
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

func (b *BackupCommand) tlsArgs(mariadb interfaces.TLSProvider) []string {
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

func (b *BackupCommand) physicalBackupArgs() []string {
	if b.BackupContentType != mariadbv1alpha1.BackupContentTypePhysical {
		return nil
	}
	var args []string
	if b.BackupFullDirPath != "" {
		args = append(args, []string{
			"--physical-backup-dir-path",
			b.BackupFullDirPath,
		}...)
	}
	if b.PhysicalBackupMeta && b.PhysicalBackupKey != nil {
		args = append(args, []string{
			"--physical-backup-meta",
			"--physical-backup-name",
			b.PhysicalBackupKey.Name,
			"--physical-backup-namespace",
			b.PhysicalBackupKey.Namespace,
		}...)
	}
	return args
}

func copyBinlogMetaCmds(sourceDir string, destDir string) []string {
	// Binlog file with the GTID coordinate is not available on the destination directory.
	// This ensures that we have access to the coordinate after restoring the backup.
	copyBinlogMetaCmd := func(binlogFileName string) string {
		sourcePath := filepath.Join(sourceDir, binlogFileName)
		destPath := filepath.Join(destDir, replication.MariaDBOperatorFileName)
		return fmt.Sprintf(`if [ -f %[1]s ]; then 
	echo "💾 Copying binlog position file '%[1]s' to '%[2]s'";
	cp %[1]s %[2]s
fi`,
			sourcePath,
			destPath,
		)
	}
	return []string{
		copyBinlogMetaCmd(replication.BinlogFileName),
		copyBinlogMetaCmd(replication.LegacyBinlogFileName),
	}
}
