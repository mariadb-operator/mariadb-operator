package pitr

import (
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

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
	logger := ctrl.Log.WithName("test")
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
			file, err := GetTargetRecoveryFile(tt.backupFiles, tt.targetRecovery, logger)
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

func mustParseDate(t *testing.T, dateString string) time.Time {
	target, err := time.Parse(timeLayout, dateString)
	if err != nil {
		t.Fatalf("unexpected error parsing date: %v", err)
	}
	return target
}
