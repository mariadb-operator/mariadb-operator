package minio

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/minio/minio-go/v7/pkg/credentials"
)

// objectLockS3 is a minimal S3 endpoint that, like a bucket with an Object Lock default retention,
// rejects PutObject and UploadPart requests that carry neither Content-MD5 nor an x-amz-checksum-* header.
type objectLockS3 struct {
	mu        sync.Mutex
	puts      int
	parts     int
	completes int
}

func (s *objectLockS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	_, _ = io.Copy(io.Discard, r.Body)

	switch {
	case r.Method == http.MethodPost && q.Has("uploads"):
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<InitiateMultipartUploadResult><Bucket>backups</Bucket><Key>k</Key>`+
			`<UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`)
		return
	case r.Method == http.MethodPost && q.Has("uploadId"):
		s.mu.Lock()
		s.completes++
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<CompleteMultipartUploadResult><Bucket>backups</Bucket><Key>k</Key>`+
			`<ETag>"etag"</ETag></CompleteMultipartUploadResult>`)
		return
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		w.WriteHeader(http.StatusNoContent)
		return
	case r.Method != http.MethodPut:
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	if !hasIntegrityHeader(r) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<Error><Code>InvalidRequest</Code>`+
			`<Message>Content-MD5 OR x-amz-checksum- HTTP header is required for Put Object requests with Object Lock parameters</Message>`+
			`</Error>`)
		return
	}

	s.mu.Lock()
	if q.Has("partNumber") {
		s.parts++
	} else {
		s.puts++
	}
	s.mu.Unlock()
	w.Header().Set("ETag", `"etag"`)
	w.WriteHeader(http.StatusOK)
}

func hasIntegrityHeader(r *http.Request) bool {
	if md5, err := base64.StdEncoding.DecodeString(r.Header.Get("Content-MD5")); err == nil && len(md5) == 16 {
		return true
	}
	for k := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-checksum-") {
			return true
		}
	}
	return false
}

func newObjectLockTestClient(t *testing.T, basePath string) (*Client, *objectLockS3) {
	t.Helper()
	fake := &objectLockS3{}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	client, err := NewMinioClient(
		basePath,
		"backups",
		strings.TrimPrefix(srv.URL, "http://"),
		WithRegion("us-east-1"),
		WithCredsProviders(&credentials.Static{
			Value: credentials.Value{
				AccessKeyID:     "test",
				SecretAccessKey: "testtesttest",
				SignerType:      credentials.SignatureV4,
			},
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	return client, fake
}

func TestPutObjectOptionsSendContentMd5(t *testing.T) {
	client := &Client{}
	opts, err := client.putObjectOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.SendContentMd5 {
		t.Error("expected SendContentMd5 to be enabled")
	}
}

func TestUploadsToObjectLockBucket(t *testing.T) {
	// Larger than minio-go's 16MiB minimum part size, so it is uploaded via multipart.
	const multipartSize = 17 * 1024 * 1024

	tests := []struct {
		name          string
		size          int
		fromFile      bool
		wantPuts      int
		wantParts     int
		wantCompletes int
	}{
		{
			name:     "PutObject single part",
			size:     1024,
			wantPuts: 1,
		},
		{
			name:     "FPutObject single part",
			size:     1024,
			fromFile: true,
			wantPuts: 1,
		},
		{
			name:          "PutObject multipart",
			size:          multipartSize,
			wantParts:     2,
			wantCompletes: 1,
		},
		{
			name:          "FPutObject multipart",
			size:          multipartSize,
			fromFile:      true,
			wantParts:     2,
			wantCompletes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			client, fake := newObjectLockTestClient(t, dir)
			data := bytes.Repeat([]byte("a"), tt.size)

			var err error
			if tt.fromFile {
				fileName := "backup.sql"
				if err := os.WriteFile(filepath.Join(dir, fileName), data, 0o600); err != nil {
					t.Fatalf("unexpected error writing file: %v", err)
				}
				err = client.FPutObjectWithOptions(context.Background(), fileName)
			} else {
				err = client.PutObjectWithOptions(context.Background(), "backup.sql", bytes.NewReader(data), int64(len(data)))
			}
			if err != nil {
				t.Fatalf("unexpected error uploading: %v", err)
			}

			if fake.puts != tt.wantPuts {
				t.Errorf("expected %d PutObject requests, got %d", tt.wantPuts, fake.puts)
			}
			if fake.parts != tt.wantParts {
				t.Errorf("expected %d UploadPart requests, got %d", tt.wantParts, fake.parts)
			}
			if fake.completes != tt.wantCompletes {
				t.Errorf("expected %d CompleteMultipartUpload requests, got %d", tt.wantCompletes, fake.completes)
			}
		})
	}
}
