package backup

import (
	"reflect"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	mdbtime "github.com/mariadb-operator/mariadb-operator/v25/pkg/time"
	ctrl "sigs.k8s.io/controller-runtime"
)

var logger = ctrl.Log.WithName("test")

func TestLogicalGetTargetFile(t *testing.T) {
	p := NewLogicalBackupProcessor()
	tests := []struct {
		name           string
		backupFiles    []string
		targetRecovery time.Time
		wantFile       string
		wantErr        bool
	}{
		{
			name:           "no backups",
			backupFiles:    []string{},
			targetRecovery: time.Now(),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "invalid backups",
			backupFiles: []string{
				"backup.foo.sql",
				"backup.bar.sql",
				"backup.sql",
			},
			targetRecovery: time.Now(),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "single backup",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
			},
			targetRecovery: time.Now(),
			wantFile:       "backup.2023-12-18T15:58:00Z.sql",
			wantErr:        false,
		},
		{
			name: "multiple backups with invalid",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.foo.sql",
				"backup.foo.sql",
			},
			targetRecovery: mustParseDate(t, "2023-12-18T15:59:00Z"),
			wantFile:       "backup.2023-12-18T15:58:00Z.sql",
			wantErr:        false,
		},
		{
			name: "fine grained",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:58:01Z.sql",
				"backup.2023-12-18T16:00:Z.sql",
			},
			targetRecovery: mustParseDate(t, "2023-12-18T15:59:00Z"),
			wantFile:       "backup.2023-12-18T15:58:01Z.sql",
			wantErr:        false,
		},
		{
			name: "target before backups",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			targetRecovery: time.UnixMilli(0),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "target after backups",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			targetRecovery: time.Now(),
			wantFile:       "backup.2023-12-18T16:13:00Z.sql",
			wantErr:        false,
		},
		{
			name: "close target",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			targetRecovery: mustParseDate(t, "2023-12-18T16:04:00Z"),
			wantFile:       "backup.2023-12-18T16:03:00Z.sql",
			wantErr:        false,
		},
		{
			name: "exact target",
			backupFiles: []string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			targetRecovery: mustParseDate(t, "2023-12-18T16:07:00Z"),
			wantFile:       "backup.2023-12-18T16:07:00Z.sql",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := p.GetBackupTargetFile(tt.backupFiles, tt.targetRecovery, logger)
			if err != nil && !tt.wantErr {
				t.Fatalf("unexpected error getting target recovery file: %v", err)
			}

			if tt.wantFile != file {
				t.Fatalf("unexpected backup target file, expected: %v got: %v", tt.wantFile, file)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestLogicalGetOldBackupFiles(t *testing.T) {
	p := NewLogicalBackupProcessor()
	previousNowFn := now
	tests := []struct {
		name         string
		nowFn        func() time.Time
		backupFiles  []string
		maxRetention time.Duration
		wantBackups  []string
	}{
		{
			name:         "no backups",
			nowFn:        testTimeFn(mustParseDate(t, "2023-12-22T22:10:00Z")),
			backupFiles:  nil,
			maxRetention: 1 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "invalid backups",
			nowFn: testTimeFn(mustParseDate(t, "2023-12-22T22:10:00Z")),
			backupFiles: []string{
				"backup.foo.sql",
				"backup.bar.sql",
				"backup.sql",
			},
			maxRetention: 1 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "no old backups",
			nowFn: testTimeFn(mustParseDate(t, "2023-12-22T22:10:00Z")),
			backupFiles: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
			maxRetention: 24 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "multiple old backups",
			nowFn: testTimeFn(mustParseDate(t, "2023-12-22T22:10:00Z")),
			backupFiles: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
			},
		},
		{
			name:  "multiple old backups with invalid",
			nowFn: testTimeFn(mustParseDate(t, "2023-12-22T22:10:00Z")),
			backupFiles: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
				"backup.foo.sql",
				"backup.bar.sql",
				"backup.sql",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
			},
		},
		{
			name:  "all old backups",
			nowFn: testTimeFn(mustParseDate(t, "2023-12-22T22:10:00Z")),
			backupFiles: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
			maxRetention: 1 * time.Hour,
			wantBackups: []string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now = tt.nowFn
			t.Cleanup(func() {
				now = previousNowFn
			})

			backups := p.GetOldBackupFiles(tt.backupFiles, tt.maxRetention, logger)
			if !reflect.DeepEqual(tt.wantBackups, backups) {
				t.Fatalf("unexpected backup files, expected: %v got: %v", tt.wantBackups, backups)
			}
		})
	}
}

func TestLogicalIsValidBackupFile(t *testing.T) {
	p := NewLogicalBackupProcessor()
	tests := []struct {
		name       string
		backupFile string
		wantValid  bool
	}{
		{
			name:       "empty",
			backupFile: "",
			wantValid:  false,
		},
		{
			name:       "no date",
			backupFile: "backup.sql",
			wantValid:  false,
		},
		{
			name:       "no prefix",
			backupFile: "2023-12-18 16:14.sql",
			wantValid:  false,
		},
		{
			name:       "no extension",
			backupFile: "backup.2023-12-18T16:14:00Z",
			wantValid:  false,
		},
		{
			name:       "invalid date",
			backupFile: "backup.2023-12-18 16:14.sql",
			wantValid:  false,
		},
		{
			name:       "invalid compression",
			backupFile: "backup.2023-12-18 16:14.foo.sql",
			wantValid:  false,
		},
		{
			name:       "valid",
			backupFile: "backup.2023-12-18T16:14:00Z.sql",
			wantValid:  true,
		},
		{
			name:       "valid with compression",
			backupFile: "backup.2023-12-18T16:14:00Z.bzip2.sql",
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := p.IsValidBackupFile(tt.backupFile)
			if tt.wantValid != valid {
				t.Fatalf("unexpected backup file validity, expected: %v got: %v", tt.wantValid, valid)
			}
		})
	}
}

func TestLogicalParseCompressionAlgorithm(t *testing.T) {
	p := NewLogicalBackupProcessor()
	tests := []struct {
		name         string
		fileName     string
		wantCompress mariadbv1alpha1.CompressAlgorithm
		wantErr      bool
	}{
		{
			name:         "empty",
			fileName:     "",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "invalid",
			fileName:     "foo",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "invalid format",
			fileName:     "backup.sql",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "no compression",
			fileName:     "backup.2023-12-22T13:00:00Z.sql",
			wantCompress: mariadbv1alpha1.CompressNone,
			wantErr:      false,
		},
		{
			name:         "invalid compression",
			fileName:     "backup.2023-12-22T13:00:00Z.foo.sql",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "legacy compression gzip",
			fileName:     "backup.2023-12-22T13:00:00Z.gzip.sql",
			wantCompress: mariadbv1alpha1.CompressGzip,
			wantErr:      false,
		},
		{
			name:         "legacy compression bzip2",
			fileName:     "backup.2023-12-22T13:00:00Z.bzip2.sql",
			wantCompress: mariadbv1alpha1.CompressBzip2,
			wantErr:      false,
		},
		{
			name:         "new format compression gz",
			fileName:     "backup.2023-12-22T13:00:00Z.sql.gz",
			wantCompress: mariadbv1alpha1.CompressGzip,
			wantErr:      false,
		},
		{
			name:         "new format compression bz2",
			fileName:     "backup.2023-12-22T13:00:00Z.sql.bz2",
			wantCompress: mariadbv1alpha1.CompressBzip2,
			wantErr:      false,
		},
		{
			name:         "new format invalid extension",
			fileName:     "backup.2023-12-22T13:00:00Z.sql.foo",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compress, err := p.ParseCompressionAlgorithm(tt.fileName)
			if tt.wantCompress != compress {
				t.Fatalf("unexpected compression algorithm, expected: %v got: %v", tt.wantCompress, compress)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestLogicalGetUncompressedBackupFile(t *testing.T) {
	p := NewLogicalBackupProcessor()
	tests := []struct {
		name         string
		fileName     string
		wantFileName string
		wantErr      bool
	}{
		{
			name:         "empty",
			fileName:     "",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "invalid",
			fileName:     "foo",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "invalid format",
			fileName:     "backup.sql",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "no compression",
			fileName:     "backup.2023-12-22T13:00:00Z.sql",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "invalid compression",
			fileName:     "backup.2023-12-22T13:00:00Z.foo.sql",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "legacy compression gzip",
			fileName:     "backup.2023-12-22T13:00:00Z.gzip.sql",
			wantFileName: "backup.2023-12-22T13:00:00Z.sql",
			wantErr:      false,
		},
		{
			name:         "legacy compression bzip2",
			fileName:     "backup.2023-12-22T13:00:00Z.bzip2.sql",
			wantFileName: "backup.2023-12-22T13:00:00Z.sql",
			wantErr:      false,
		},
		{
			name:         "new format compression gz",
			fileName:     "backup.2023-12-22T13:00:00Z.sql.gz",
			wantFileName: "backup.2023-12-22T13:00:00Z.sql",
			wantErr:      false,
		},
		{
			name:         "new format compression bz2",
			fileName:     "backup.2023-12-22T13:00:00Z.sql.bz2",
			wantFileName: "backup.2023-12-22T13:00:00Z.sql",
			wantErr:      false,
		},
		{
			name:         "new format invalid extension",
			fileName:     "backup.2023-12-22T13:00:00Z.sql.foo",
			wantFileName: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName, err := p.GetUncompressedBackupFile(tt.fileName)
			if tt.wantFileName != fileName {
				t.Fatalf("unexpected uncompressed file, expected: %v got: %v", tt.wantFileName, fileName)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestPhysicalGetTargetFile(t *testing.T) {
	p := NewPhysicalBackupProcessor()
	tests := []struct {
		name           string
		backupFiles    []string
		targetRecovery time.Time
		wantFile       string
		wantErr        bool
	}{
		{
			name:           "no backups",
			backupFiles:    []string{},
			targetRecovery: time.Now(),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "invalid backups",
			backupFiles: []string{
				"physicalbackup.foo.xb",
				"physicalbackup.bar.xb",
				"physicalbackup.xb",
			},
			targetRecovery: time.Now(),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "single backup",
			backupFiles: []string{
				"physicalbackup-20231218155800.xb",
			},
			targetRecovery: time.Now(),
			wantFile:       "physicalbackup-20231218155800.xb",
			wantErr:        false,
		},
		{
			name: "multiple backups with invalid",
			backupFiles: []string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup.foo.xb",
				"physicalbackup.bar.xb",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218155900"),
			wantFile:       "physicalbackup-20231218155800.xb",
			wantErr:        false,
		},
		{
			name: "fine grained",
			backupFiles: []string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218155801.xb",
				"physicalbackup-20231218160000.xb",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218155900"),
			wantFile:       "physicalbackup-20231218155801.xb",
			wantErr:        false,
		},
		{
			name: "target before backups",
			backupFiles: []string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218155900.xb",
			},
			targetRecovery: time.UnixMilli(0),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "target after backups",
			backupFiles: []string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218161300.xb",
			},
			targetRecovery: time.Now(),
			wantFile:       "physicalbackup-20231218161300.xb",
			wantErr:        false,
		},
		{
			name: "exact target",
			backupFiles: []string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218160700.xb",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218160700"),
			wantFile:       "physicalbackup-20231218160700.xb",
			wantErr:        false,
		},
		{
			name: "prefixes",
			backupFiles: []string{
				"mariadb/physicalbackup-20231218155800.xb",
				"mariadb/physicalbackup-20231218160700.xb",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218160700"),
			wantFile:       "mariadb/physicalbackup-20231218160700.xb",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := p.GetBackupTargetFile(tt.backupFiles, tt.targetRecovery, logger)
			if err != nil && !tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantFile != file {
				t.Fatalf("unexpected file, expected: %s got: %s", tt.wantFile, file)
			}
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPhysicalGetOldBackupFiles(t *testing.T) {
	p := NewPhysicalBackupProcessor()
	previousNowFn := now
	tests := []struct {
		name         string
		nowFn        func() time.Time
		backupFiles  []string
		maxRetention time.Duration
		wantBackups  []string
	}{
		{
			name:         "no backups",
			nowFn:        testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles:  nil,
			maxRetention: 1 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "invalid backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"physicalbackup.foo.xb",
				"physicalbackup.bar.xb",
				"physicalbackup.xb",
			},
			maxRetention: 1 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "no old backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
			maxRetention: 24 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "multiple old backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
			},
		},
		{
			name:  "multiple old backups with invalid",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
				"physicalbackup-foo.xb",
				"physicalbackup-bar.xb",
				"physicalbackup-sql",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
			},
		},
		{
			name:  "all old backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
			maxRetention: 1 * time.Hour,
			wantBackups: []string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
		},
		{
			name:  "prefix",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"mariadb/physicalbackup-20231222130000.xb",
				"mariadb/physicalbackup-20231222140000.xb",
				"mariadb/physicalbackup-20231222150000.xb",
				"mariadb/physicalbackup-20231222160000.xb",
				"mariadb/physicalbackup-20231222170000.xb",
				"mariadb/physicalbackup-20231222180000.xb",
				"mariadb/physicalbackup-20231222190000.xb",
				"mariadb/physicalbackup-20231222200000.xb",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"mariadb/physicalbackup-20231222130000.xb",
				"mariadb/physicalbackup-20231222140000.xb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now = tt.nowFn
			t.Cleanup(func() {
				now = previousNowFn
			})

			backups := p.GetOldBackupFiles(tt.backupFiles, tt.maxRetention, logger)
			if !reflect.DeepEqual(tt.wantBackups, backups) {
				t.Fatalf("unexpected backup files, expected: %v got: %v", tt.wantBackups, backups)
			}
		})
	}
}

func TestPhysicalIsValidBackupFile(t *testing.T) {
	p := NewPhysicalBackupProcessor()
	tests := []struct {
		name       string
		backupFile string
		wantValid  bool
	}{
		{
			name:       "empty",
			backupFile: "",
			wantValid:  false,
		},
		{
			name:       "no date",
			backupFile: "physicalbackup.xb",
			wantValid:  false,
		},
		{
			name:       "no prefix",
			backupFile: "202312181614.xb",
			wantValid:  false,
		},
		{
			name:       "no extension",
			backupFile: "physicalbackup-20231218161400",
			wantValid:  false,
		},
		{
			name:       "invalid date",
			backupFile: "physicalbackup-202312181614.xb",
			wantValid:  false,
		},
		{
			name:       "invalid compression",
			backupFile: "physicalbackup-202312181614.xb.foo",
			wantValid:  false,
		},
		{
			name:       "valid",
			backupFile: "physicalbackup-20231218161400.xb",
			wantValid:  true,
		},
		{
			name:       "valid with gzip",
			backupFile: "physicalbackup-20231218161400.xb.gz",
			wantValid:  true,
		},
		{
			name:       "valid with bzip2",
			backupFile: "physicalbackup-20231218161400.xb.bz2",
			wantValid:  true,
		},
		{
			name:       "valid with prefix",
			backupFile: "mariadb/physicalbackup-20231218161400.xb",
			wantValid:  true,
		},
		{
			name:       "valid with prefix and compression",
			backupFile: "mariadb/physicalbackup-20231218161400.xb.gz",
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := p.IsValidBackupFile(tt.backupFile)
			if tt.wantValid != valid {
				t.Fatalf("unexpected backup file validity, expected: %v got: %v", tt.wantValid, valid)
			}
		})
	}
}

func TestPhysicalParseCompressionAlgorithm(t *testing.T) {
	p := NewPhysicalBackupProcessor()
	tests := []struct {
		name         string
		fileName     string
		wantCompress mariadbv1alpha1.CompressAlgorithm
		wantErr      bool
	}{
		{
			name:         "empty",
			fileName:     "",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "invalid",
			fileName:     "foo",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "no compression",
			fileName:     "physicalbackup-20231222130000.xb",
			wantCompress: mariadbv1alpha1.CompressNone,
			wantErr:      false,
		},
		{
			name:         "invalid compression",
			fileName:     "physicalbackup-20231222130000.xb.foo",
			wantCompress: mariadbv1alpha1.CompressAlgorithm(""),
			wantErr:      true,
		},
		{
			name:         "gzip",
			fileName:     "physicalbackup-20231222130000.xb.gz",
			wantCompress: mariadbv1alpha1.CompressGzip,
			wantErr:      false,
		},
		{
			name:         "bzip2",
			fileName:     "physicalbackup-20231222130000.xb.bz2",
			wantCompress: mariadbv1alpha1.CompressBzip2,
			wantErr:      false,
		},
		{
			name:         "gzip and prefix",
			fileName:     "mariadb/physicalbackup-20231222130000.xb.gz",
			wantCompress: mariadbv1alpha1.CompressGzip,
			wantErr:      false,
		},
		{
			name:         "bzip2 and prefix",
			fileName:     "mariadb/physicalbackup-20231222130000.xb.bz2",
			wantCompress: mariadbv1alpha1.CompressBzip2,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compress, err := p.ParseCompressionAlgorithm(tt.fileName)
			if tt.wantCompress != compress {
				t.Fatalf("unexpected compression algorithm, expected: %v got: %v", tt.wantCompress, compress)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestPhysicalGetUncompressedBackupFile(t *testing.T) {
	p := NewPhysicalBackupProcessor()
	tests := []struct {
		name         string
		fileName     string
		wantFileName string
		wantErr      bool
	}{
		{
			name:         "empty",
			fileName:     "",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "invalid",
			fileName:     "foo",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "invalid format",
			fileName:     "physicalbackup.xb",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "no compression",
			fileName:     "physicalbackup-20231222130000.xb",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "invalid compression",
			fileName:     "physicalbackup-20231222130000.xb.foo",
			wantFileName: "",
			wantErr:      true,
		},
		{
			name:         "gzip",
			fileName:     "physicalbackup-20231222130000.xb.gz",
			wantFileName: "physicalbackup-20231222130000.xb",
			wantErr:      false,
		},
		{
			name:         "bzip2",
			fileName:     "physicalbackup-20231222130000.xb.bz2",
			wantFileName: "physicalbackup-20231222130000.xb",
			wantErr:      false,
		},
		{
			name:         "prefix and gzip",
			fileName:     "mariadb/physicalbackup-20231222130000.xb.gz",
			wantFileName: "mariadb/physicalbackup-20231222130000.xb",
			wantErr:      false,
		},
		{
			name:         "prefix and bzip2",
			fileName:     "mariadb/physicalbackup-20231222130000.xb.bz2",
			wantFileName: "mariadb/physicalbackup-20231222130000.xb",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName, err := p.GetUncompressedBackupFile(tt.fileName)
			if tt.wantFileName != fileName {
				t.Fatalf("unexpected uncompressed file, expected: %v got: %v", tt.wantFileName, fileName)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestSnapshotGetTargetFile(t *testing.T) {
	p := NewPhysicalBackupProcessor(
		WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)
	tests := []struct {
		name           string
		backupFiles    []string
		targetRecovery time.Time
		wantFile       string
		wantErr        bool
	}{
		{
			name:           "no backups",
			backupFiles:    []string{},
			targetRecovery: time.Now(),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "invalid backups",
			backupFiles: []string{
				"snapshot.foo",
				"snapshot.bar",
				"snapshot",
			},
			targetRecovery: time.Now(),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "single backup",
			backupFiles: []string{
				"snapshot-20231218155800",
			},
			targetRecovery: time.Now(),
			wantFile:       "snapshot-20231218155800",
			wantErr:        false,
		},
		{
			name: "multiple backups with invalid",
			backupFiles: []string{
				"snapshot-20231218155800",
				"snapshot.foo",
				"snapshot.bar",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218155900"),
			wantFile:       "snapshot-20231218155800",
			wantErr:        false,
		},
		{
			name: "fine grained",
			backupFiles: []string{
				"snapshot-20231218155800",
				"snapshot-20231218155801",
				"snapshot-20231218160000",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218155900"),
			wantFile:       "snapshot-20231218155801",
			wantErr:        false,
		},
		{
			name: "target before backups",
			backupFiles: []string{
				"snapshot-20231218155800",
				"snapshot-20231218155900",
			},
			targetRecovery: time.UnixMilli(0),
			wantFile:       "",
			wantErr:        true,
		},
		{
			name: "target after backups",
			backupFiles: []string{
				"snapshot-20231218155800",
				"snapshot-20231218161300",
			},
			targetRecovery: time.Now(),
			wantFile:       "snapshot-20231218161300",
			wantErr:        false,
		},
		{
			name: "exact target",
			backupFiles: []string{
				"snapshot-20231218155800",
				"snapshot-20231218160700",
			},
			targetRecovery: mustParseMariadbDate(t, "20231218160700"),
			wantFile:       "snapshot-20231218160700",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := p.GetBackupTargetFile(tt.backupFiles, tt.targetRecovery, logger)
			if err != nil && !tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantFile != file {
				t.Fatalf("unexpected file, expected: %s got: %s", tt.wantFile, file)
			}
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSnapshotGetOldBackupFiles(t *testing.T) {
	p := NewPhysicalBackupProcessor(
		WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)
	previousNowFn := now
	tests := []struct {
		name         string
		nowFn        func() time.Time
		backupFiles  []string
		maxRetention time.Duration
		wantBackups  []string
	}{
		{
			name:         "no backups",
			nowFn:        testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles:  nil,
			maxRetention: 1 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "invalid backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"snapshot.foo",
				"snapshot.bar",
				"snapshot",
			},
			maxRetention: 1 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "no old backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
			maxRetention: 24 * time.Hour,
			wantBackups:  nil,
		},
		{
			name:  "multiple old backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
			},
		},
		{
			name:  "multiple old backups with invalid",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
				"snapshot-foo",
				"snapshot-bar",
				"snapshot-sql",
			},
			maxRetention: 8 * time.Hour,
			wantBackups: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
			},
		},
		{
			name:  "all old backups",
			nowFn: testTimeFn(mustParseMariadbDate(t, "20231222221000")),
			backupFiles: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
			maxRetention: 1 * time.Hour,
			wantBackups: []string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now = tt.nowFn
			t.Cleanup(func() {
				now = previousNowFn
			})

			backups := p.GetOldBackupFiles(tt.backupFiles, tt.maxRetention, logger)
			if !reflect.DeepEqual(tt.wantBackups, backups) {
				t.Fatalf("unexpected backup files, expected: %v got: %v", tt.wantBackups, backups)
			}
		})
	}
}

func TestSnapshotIsValidBackupFile(t *testing.T) {
	p := NewPhysicalBackupProcessor(
		WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)
	tests := []struct {
		name       string
		backupFile string
		wantValid  bool
	}{
		{
			name:       "empty",
			backupFile: "",
			wantValid:  false,
		},
		{
			name:       "no date",
			backupFile: "snapshot",
			wantValid:  false,
		},
		{
			name:       "no prefix",
			backupFile: "202312181614",
			wantValid:  false,
		},
		{
			name:       "invalid date",
			backupFile: "snapshot-202312181614",
			wantValid:  false,
		},
		{
			name:       "valid",
			backupFile: "snapshot-20231218161400",
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := p.IsValidBackupFile(tt.backupFile)
			if tt.wantValid != valid {
				t.Fatalf("unexpected backup file validity, expected: %v got: %v", tt.wantValid, valid)
			}
		})
	}
}

func testTimeFn(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func mustParseDate(t *testing.T, dateString string) time.Time {
	t.Helper()
	target, err := time.Parse(timeLayout, dateString)
	if err != nil {
		t.Fatalf("unexpected error parsing date: %v", err)
	}
	return target
}

func mustParseMariadbDate(t *testing.T, dateString string) time.Time {
	t.Helper()
	target, err := mdbtime.Parse(dateString)
	if err != nil {
		t.Fatalf("unexpected error parsing date: %v", err)
	}
	return target
}
