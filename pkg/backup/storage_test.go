package backup

import (
	"encoding/base64"
	"testing"
)

func TestS3PrefixedFile(t *testing.T) {
	tests := []struct {
		name          string
		backupStorage *S3BackupStorage
		fileName      string
		wantFileName  string
	}{
		{
			name:          "no prefix",
			backupStorage: &S3BackupStorage{},
			fileName:      "backup.2023-12-18T16:14:00Z.sql",
			wantFileName:  "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:          "no prefix with file path",
			backupStorage: &S3BackupStorage{},
			fileName:      "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName:  "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with file path",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with trailing slash",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb/",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with trailing slash and file path",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb/",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "nested prefix",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "nested prefix with file path",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "already prefixed",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := tt.backupStorage.prefixedFileName(tt.fileName)
			if fileName != tt.wantFileName {
				t.Errorf("unexpected S3 file name, got: \"%s\" want: \"%s\"", fileName, tt.wantFileName)
			}
		})
	}
}

func TestS3UnprefixedFile(t *testing.T) {
	tests := []struct {
		name          string
		backupStorage *S3BackupStorage
		fileName      string
		wantFileName  string
	}{
		{
			name:          "no prefix",
			backupStorage: &S3BackupStorage{},
			fileName:      "backup.2023-12-18T16:14:00Z.sql",
			wantFileName:  "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:          "no prefix with file path",
			backupStorage: &S3BackupStorage{},
			fileName:      "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName:  "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with file path",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with trailing slash",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb/",
				},
			},
			fileName:     "mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "nested prefix",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			fileName:     "backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "already unprefixed",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := tt.backupStorage.unprefixedFilename(tt.fileName)
			if fileName != tt.wantFileName {
				t.Errorf("unexpected S3 file name, got: \"%s\" want: \"%s\"", fileName, tt.wantFileName)
			}
		})
	}
}

func TestS3Prefix(t *testing.T) {
	tests := []struct {
		name          string
		backupStorage *S3BackupStorage
		wantPrefix    string
	}{
		{
			name:          "no prefix",
			backupStorage: &S3BackupStorage{},
			wantPrefix:    "",
		},
		{
			name: "root",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "/",
				},
			},
			wantPrefix: "",
		},
		{
			name: "no trailing slash",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb",
				},
			},
			wantPrefix: "mariadb/",
		},
		{
			name: "trailing slash",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "mariadb/",
				},
			},
			wantPrefix: "mariadb/",
		},
		{
			name: "nested without trailing slash",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			wantPrefix: "backups/production/mariadb/",
		},
		{
			name: "nested with trailing slash",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					Prefix: "backups/production/mariadb/",
				},
			},
			wantPrefix: "backups/production/mariadb/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := tt.backupStorage.getPrefix()
			if prefix != tt.wantPrefix {
				t.Errorf("unexpected S3 prefix, got: \"%s\" want: \"%s\"", prefix, tt.wantPrefix)
			}
		})
	}
}

func TestS3GetSSEC(t *testing.T) {
	// Valid 32-byte key for AES-256
	validKey := make([]byte, 32)
	for i := range validKey {
		validKey[i] = byte(i)
	}
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)

	// Invalid key (not 32 bytes)
	invalidKey := make([]byte, 16)
	invalidKeyBase64 := base64.StdEncoding.EncodeToString(invalidKey)

	tests := []struct {
		name          string
		backupStorage *S3BackupStorage
		wantNil       bool
		wantErr       bool
	}{
		{
			name:          "no SSE-C key",
			backupStorage: &S3BackupStorage{},
			wantNil:       true,
			wantErr:       false,
		},
		{
			name: "empty SSE-C key",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					SSECCustomerKey: "",
				},
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "valid SSE-C key",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					SSECCustomerKey: validKeyBase64,
				},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "invalid base64",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					SSECCustomerKey: "not-valid-base64!!!",
				},
			},
			wantNil: true,
			wantErr: true,
		},
		{
			name: "invalid key length (not 32 bytes)",
			backupStorage: &S3BackupStorage{
				S3BackupStorageOpts: S3BackupStorageOpts{
					SSECCustomerKey: invalidKeyBase64,
				},
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sse, err := tt.backupStorage.getSSEC()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if sse != nil {
					t.Error("expected nil SSE-C, got non-nil")
				}
			} else {
				if sse == nil {
					t.Error("expected non-nil SSE-C, got nil")
				}
			}
		})
	}
}
