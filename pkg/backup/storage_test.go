package backup

import "testing"

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
