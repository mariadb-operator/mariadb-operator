package backup

import (
	"reflect"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

var logger = ctrl.Log.WithName("test")

func TestIsValidBackupFile(t *testing.T) {
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
			name:       "valid",
			backupFile: "backup.2023-12-18T16:14:00Z.sql",
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := IsValidBackupFile(tt.backupFile)
			if tt.wantValid != valid {
				t.Fatalf("unexpected backup file validity, expected: %v got: %v", tt.wantValid, valid)
			}
		})
	}
}

func TestGetTargerRecoveryFile(t *testing.T) {
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
			targetRecovery: time.UnixMilli(0),
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
			wantFile:       "backup.2023-12-18T15:58:00Z.sql",
			wantErr:        false,
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
			file, err := GetBackupTargetFile(tt.backupFiles, tt.targetRecovery, logger)
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

func TestGetBackupFilesToDelete(t *testing.T) {
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

			backups := GetOldBackupFiles(tt.backupFiles, tt.maxRetention, logger)
			if !reflect.DeepEqual(tt.wantBackups, backups) {
				t.Fatalf("unexpected backup files, expected: %v got: %v", tt.wantBackups, backups)
			}
		})
	}
}

func testTimeFn(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func mustParseDate(t *testing.T, dateString string) time.Time {
	target, err := time.Parse(timeLayout, dateString)
	if err != nil {
		t.Fatalf("unexpected error parsing date: %v", err)
	}
	return target
}
