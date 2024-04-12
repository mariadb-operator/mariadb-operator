package backup

import "testing"

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
			name:          "root",
			backupStorage: &S3BackupStorage{},
			wantPrefix:    "",
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
