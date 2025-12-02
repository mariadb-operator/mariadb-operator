package minio

import (
	"encoding/base64"
	"testing"
)

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
		name    string
		client  *Client
		wantNil bool
		wantErr bool
	}{
		{
			name:    "no SSE-C key",
			client:  &Client{},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "empty SSE-C key",
			client: &Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: "",
				},
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "valid SSE-C key",
			client: &Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: validKeyBase64,
				},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "invalid base64",
			client: &Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: invalidKeyBase64,
				},
			},
			wantNil: true,
			wantErr: true,
		},
		{
			name: "invalid base64 (not 32 bytes)",
			client: &Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: "not-valid-base64!!!",
				},
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sse, err := tt.client.getSSEC()

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
