package minio

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
  "github.com/minio/minio-go/v7/pkg/credentials"
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

func TestDualStackEndpoint(t *testing.T) {
	bucket := "test-bucket"
	tests := []struct {
		name          string
		endpoint      string
		region        string
		wantDualStack bool
		wantHost      string
	}{
		{
			name:          "standard AWS regional endpoint",
			endpoint:      "s3.eu-central-1.amazonaws.com",
			region:        "eu-central-1",
			wantDualStack: false,
			wantHost:      bucket + ".s3.eu-central-1.amazonaws.com",
		},
		{
			name:          "global AWS endpoint",
			endpoint:      "s3.amazonaws.com",
			region:        "eu-central-1",
			wantDualStack: false,
			wantHost:      bucket + ".s3.eu-central-1.amazonaws.com",
		},
		{
			name:          "dual-stack AWS endpoint",
			endpoint:      "s3.dualstack.eu-central-1.amazonaws.com",
			region:        "eu-central-1",
			wantDualStack: true,
			wantHost:      bucket + ".s3.dualstack.eu-central-1.amazonaws.com",
		},
		{
			name:          "dual-stack AWS endpoint with port",
			endpoint:      "s3.dualstack.eu-central-1.amazonaws.com:443",
			region:        "eu-central-1",
			wantDualStack: true,
			wantHost:      bucket + ".s3.dualstack.eu-central-1.amazonaws.com",
		},
		{
			name:          "dual-stack AWS China endpoint",
			endpoint:      "s3.dualstack.cn-north-1.amazonaws.com.cn",
			region:        "cn-north-1",
			wantDualStack: true,
			wantHost:      bucket + ".s3.dualstack.cn-north-1.amazonaws.com.cn",
		},
		{
			name:          "FIPS dual-stack AWS endpoint",
			endpoint:      "s3-fips.dualstack.us-east-1.amazonaws.com",
			region:        "us-east-1",
			wantDualStack: true,
			wantHost:      bucket + ".s3-fips.dualstack.us-east-1.amazonaws.com",
		},
		{
			name:          "AWS VPC interface endpoint",
			endpoint:      "bucket.vpce-1a2b3c4d-5e6f.s3.eu-central-1.vpce.amazonaws.com",
			region:        "eu-central-1",
			wantDualStack: false,
			wantHost:      bucket + ".bucket.vpce-1a2b3c4d-5e6f.s3.eu-central-1.vpce.amazonaws.com",
		},
		{
			name:          "non-AWS endpoint with dualstack label",
			endpoint:      "minio.dualstack.internal:9000",
			region:        "us-east-1",
			wantDualStack: false,
			wantHost:      "minio.dualstack.internal:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewMinioClient(
				"",
				bucket,
				tt.endpoint,
				WithTLS(true),
				WithRegion(tt.region),
				WithCredsProviders(&credentials.Static{
					Value: credentials.Value{
						AccessKeyID:     "test",
						SecretAccessKey: "test",
						SignerType:      credentials.SignatureDefault,
					},
				}),
			)
			if err != nil {
				t.Fatalf("unexpected error creating client: %v", err)
			}

			if got := isDualStackEndpoint(client.EndpointURL()); got != tt.wantDualStack {
				t.Errorf("unexpected dual-stack decision for %q, got: %t want: %t", tt.endpoint, got, tt.wantDualStack)
			}

			// With a region set, presigning builds the target URL without any network call.
			u, err := client.PresignedGetObject(context.Background(), bucket, "backup.sql", time.Minute, nil)
			if err != nil {
				t.Fatalf("unexpected error presigning URL: %v", err)
			}
			if u.Host != tt.wantHost {
				t.Errorf("unexpected request host for %q, got: %s want: %s", tt.endpoint, u.Host, tt.wantHost)
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

func TestNewMinioClientFromS3ConfigOptionalCACert(t *testing.T) {
	tests := []struct {
		name        string
		tls         *v1alpha1.TLSConfig
		wantTLS     bool
		wantCACerts bool
	}{
		{
			name:        "no TLS config",
			tls:         nil,
			wantTLS:     false,
			wantCACerts: false,
		},
		{
			name:        "TLS disabled",
			tls:         &v1alpha1.TLSConfig{Enabled: false},
			wantTLS:     false,
			wantCACerts: false,
		},
		{
			name:        "TLS enabled without CA bundle",
			tls:         &v1alpha1.TLSConfig{Enabled: true},
			wantTLS:     true,
			wantCACerts: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A nil backing client is enough: none of these cases may resolve a Secret.
			refResolver := refresolver.New(nil)
			s3 := v1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.example.com",
				TLS:      tt.tls,
			}

			client, err := NewMinioClientFromS3Config(t.Context(), *refResolver, s3, "", "test-namespace")
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if client.TLS != tt.wantTLS {
				t.Errorf("expected TLS %v, got %v", tt.wantTLS, client.TLS)
			}
			if gotCACerts := client.CACertBytes != nil; gotCACerts != tt.wantCACerts {
				t.Errorf("expected CA cert bytes %v, got %v", tt.wantCACerts, gotCACerts)
			}
		})
	}
}
