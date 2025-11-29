package minio

import "testing"

func TestPrefixedFile(t *testing.T) {
	tests := []struct {
		name         string
		client       *Client
		fileName     string
		wantFileName string
	}{
		{
			name:         "no prefix",
			client:       &Client{},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:         "no prefix with file path",
			client:       &Client{},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with file path",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with trailing slash",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with trailing slash and file path",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "nested prefix",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "nested prefix with file path",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "already prefixed",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "mariadb/backup.2023-12-18T16:14:00Z.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := tt.client.PrefixedFileName(tt.fileName)
			if fileName != tt.wantFileName {
				t.Errorf("unexpected S3 file name, got: %s want: %s", fileName, tt.wantFileName)
			}
		})
	}
}

func TestUnprefixedFile(t *testing.T) {
	tests := []struct {
		name         string
		client       *Client
		fileName     string
		wantFileName string
	}{
		{
			name:         "no prefix",
			client:       &Client{},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:         "no prefix with file path",
			client:       &Client{},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with file path",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "prefix with trailing slash",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			fileName:     "mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "nested prefix",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			fileName:     "backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name: "already unprefixed",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			fileName:     "backup.2023-12-18T16:14:00Z.sql",
			wantFileName: "backup.2023-12-18T16:14:00Z.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := tt.client.UnprefixedFilename(tt.fileName)
			if fileName != tt.wantFileName {
				t.Errorf("unexpected S3 file name, got: %s want: %s", fileName, tt.wantFileName)
			}
		})
	}
}

func TestPrefix(t *testing.T) {
	tests := []struct {
		name       string
		client     *Client
		wantPrefix string
	}{
		{
			name:       "no prefix",
			client:     &Client{},
			wantPrefix: "",
		},
		{
			name: "root",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "/",
				},
			},
			wantPrefix: "",
		},
		{
			name: "no trailing slash",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			wantPrefix: "mariadb/",
		},
		{
			name: "trailing slash",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			wantPrefix: "mariadb/",
		},
		{
			name: "nested without trailing slash",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			wantPrefix: "backups/production/mariadb/",
		},
		{
			name: "nested with trailing slash",
			client: &Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb/",
				},
			},
			wantPrefix: "backups/production/mariadb/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := tt.client.GetPrefix()
			if prefix != tt.wantPrefix {
				t.Errorf("unexpected S3 prefix, got: %s want: %s", prefix, tt.wantPrefix)
			}
		})
	}
}
