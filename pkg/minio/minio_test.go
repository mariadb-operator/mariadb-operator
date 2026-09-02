package minio

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"

	miniogo "github.com/minio/minio-go/v7"
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

func TestPutObjectMultipartStreamCompletesUnknownSize(t *testing.T) {
	uploader := &fakeMultipartUploader{}
	err := putObjectMultipartStream(
		context.Background(),
		uploader,
		"bucket",
		"backup.xb.gz",
		strings.NewReader("backup payload"),
		5*1024*1024,
		miniogo.PutObjectOptions{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploader.aborted {
		t.Fatal("expected successful upload not to be aborted")
	}
	if !uploader.completed {
		t.Fatal("expected multipart upload to be completed")
	}
	if len(uploader.uploadedParts) != 1 {
		t.Fatalf("unexpected uploaded parts: got %d want 1", len(uploader.uploadedParts))
	}
	if string(uploader.uploadedParts[0]) != "backup payload" {
		t.Fatalf("unexpected uploaded payload: %q", string(uploader.uploadedParts[0]))
	}
}

func TestPutObjectMultipartStreamAbortsWithFreshContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	putPartErr := errors.New("upload part failed")
	uploader := &fakeMultipartUploader{
		putPartErr: putPartErr,
	}
	err := putObjectMultipartStream(
		ctx,
		uploader,
		"bucket",
		"backup.xb.gz",
		strings.NewReader("backup payload"),
		5*1024*1024,
		miniogo.PutObjectOptions{},
	)
	if !errors.Is(err, putPartErr) {
		t.Fatalf("expected put part error, got: %v", err)
	}
	if !uploader.aborted {
		t.Fatal("expected multipart upload to be aborted")
	}
	if uploader.abortContextErr != nil {
		t.Fatalf("expected abort to use a fresh context, got: %v", uploader.abortContextErr)
	}
}

type fakeMultipartUploader struct {
	uploadedParts   [][]byte
	completed       bool
	aborted         bool
	abortContextErr error
	putPartErr      error
}

func (f *fakeMultipartUploader) NewMultipartUpload(_ context.Context, _, _ string, _ miniogo.PutObjectOptions) (string, error) {
	return "upload-id", nil
}

func (f *fakeMultipartUploader) PutObjectPart(_ context.Context, _, _, _ string, partID int, data io.Reader, size int64,
	_ miniogo.PutObjectPartOptions) (miniogo.ObjectPart, error) {
	if f.putPartErr != nil {
		return miniogo.ObjectPart{}, f.putPartErr
	}
	part, err := io.ReadAll(data)
	if err != nil {
		return miniogo.ObjectPart{}, err
	}
	if int64(len(part)) != size {
		return miniogo.ObjectPart{}, errors.New("part size mismatch")
	}
	f.uploadedParts = append(f.uploadedParts, part)
	return miniogo.ObjectPart{
		ETag:       "etag",
		PartNumber: partID,
	}, nil
}

func (f *fakeMultipartUploader) CompleteMultipartUpload(_ context.Context, _, _, _ string, _ []miniogo.CompletePart,
	_ miniogo.PutObjectOptions) (miniogo.UploadInfo, error) {
	f.completed = true
	return miniogo.UploadInfo{}, nil
}

func (f *fakeMultipartUploader) AbortMultipartUpload(ctx context.Context, _, _, _ string) error {
	f.aborted = true
	f.abortContextErr = ctx.Err()
	return nil
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
