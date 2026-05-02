package compression

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
)

func TestBackupCompressors(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	// The dump container always writes a plain ".sql" file regardless of compression. The
	// compressor produces a sibling ".sql.<ext>" file and removes the source. NopBackupCompressor
	// keeps the original ".sql" file as both source and final.
	tests := []struct {
		name            string
		newCompressorFn func(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) BackupCompressor
		plainFileName   string
		wantFinalName   string
	}{
		{
			name:            "nop",
			newCompressorFn: NewNopBackupCompressor,
			plainFileName:   "backup.2023-12-18T16:14:00Z.sql",
			wantFinalName:   "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:            "gzip",
			newCompressorFn: NewGzipBackupCompressor,
			plainFileName:   "backup.2023-12-18T16:14:00Z.sql",
			wantFinalName:   "backup.2023-12-18T16:14:00Z.sql.gz",
		},
		{
			name:            "bzip2",
			newCompressorFn: NewBzip2BackupCompressor,
			plainFileName:   "backup.2023-12-18T16:14:00Z.sql",
			wantFinalName:   "backup.2023-12-18T16:14:00Z.sql.bz2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "backup_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(dir)

			compressor := tt.newCompressorFn(dir, processor.GetUncompressedBackupFile, logger)

			plainFilePath := filepath.Join(dir, tt.plainFileName)
			if err := os.WriteFile(plainFilePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			finalName, err := compressor.Compress(tt.plainFileName)
			if err != nil {
				t.Fatalf("Failed to compress test file: %v", err)
			}
			if finalName != tt.wantFinalName {
				t.Fatalf("Compress returned unexpected filename: got %q want %q", finalName, tt.wantFinalName)
			}

			finalPath := filepath.Join(dir, finalName)
			if _, err := os.Stat(finalPath); err != nil {
				t.Fatalf("Compressed file not found at %s: %v", finalPath, err)
			}
			if tt.name != "nop" {
				if _, err := os.Stat(plainFilePath); !os.IsNotExist(err) {
					t.Fatalf("Plain source %s should be removed after compression, got err=%v", plainFilePath, err)
				}
			}

			decompressedFileName, err := compressor.Decompress(finalName)
			if err != nil {
				t.Fatalf("Failed to decompress test file: %v", err)
			}

			decompressedContent, err := os.ReadFile(decompressedFileName)
			if err != nil {
				t.Fatalf("Failed to read decompressed file: %v", err)
			}
			if string(decompressedContent) != content {
				t.Errorf("Decompressed content does not match original content:\nGot: %s\nWant: %s", decompressedContent, content)
			}
		})
	}
}

// TestBackupCompressorIdempotent verifies the operator-backup container can crash after a
// successful compress (e.g. S3 push fails), be restarted by the kubelet under
// RestartPolicy: OnFailure, and re-enter Compress() without producing a doubly-compressed file.
func TestBackupCompressorIdempotent(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	tests := []struct {
		name            string
		newCompressorFn func(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) BackupCompressor
		plainFileName   string
		wantFinalName   string
	}{
		{
			name:            "gzip",
			newCompressorFn: NewGzipBackupCompressor,
			plainFileName:   "backup.2023-12-18T16:14:00Z.sql",
			wantFinalName:   "backup.2023-12-18T16:14:00Z.sql.gz",
		},
		{
			name:            "bzip2",
			newCompressorFn: NewBzip2BackupCompressor,
			plainFileName:   "backup.2023-12-18T16:14:00Z.sql",
			wantFinalName:   "backup.2023-12-18T16:14:00Z.sql.bz2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "backup_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(dir)

			compressor := tt.newCompressorFn(dir, processor.GetUncompressedBackupFile, logger)

			plainFilePath := filepath.Join(dir, tt.plainFileName)
			if err := os.WriteFile(plainFilePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// First compress.
			if _, err := compressor.Compress(tt.plainFileName); err != nil {
				t.Fatalf("First compress failed: %v", err)
			}
			finalPath := filepath.Join(dir, tt.wantFinalName)
			firstSize, err := fileSize(finalPath)
			if err != nil {
				t.Fatalf("Failed to stat compressed file after first compress: %v", err)
			}

			// Simulate an operator-backup container restart: same target file is read again, the
			// stale plain ".sql" source is no longer present, and the ".sql.<ext>" compressed
			// output is. Re-entering Compress() must NOT wrap it in another layer.
			finalName, err := compressor.Compress(tt.plainFileName)
			if err != nil {
				t.Fatalf("Idempotent compress failed: %v", err)
			}
			if finalName != tt.wantFinalName {
				t.Fatalf("Idempotent compress returned unexpected filename: got %q want %q", finalName, tt.wantFinalName)
			}
			secondSize, err := fileSize(finalPath)
			if err != nil {
				t.Fatalf("Failed to stat compressed file after idempotent compress: %v", err)
			}
			if firstSize != secondSize {
				t.Fatalf("Compress was not idempotent: file size changed from %d to %d (likely double compression)",
					firstSize, secondSize)
			}

			// And the decompressed bytes should still match the original content (single layer of
			// compression, not nested).
			decompressedFileName, err := compressor.Decompress(finalName)
			if err != nil {
				t.Fatalf("Failed to decompress after idempotent compress: %v", err)
			}
			decompressedContent, err := os.ReadFile(decompressedFileName)
			if err != nil {
				t.Fatalf("Failed to read decompressed file: %v", err)
			}
			if string(decompressedContent) != content {
				t.Errorf("Decompressed content after idempotent compress does not match original:\nGot: %s\nWant: %s",
					decompressedContent, content)
			}
		})
	}
}

// TestBackupCompressorIdempotentWithStalePlainSource simulates a crash between Rename and Remove:
// both the compressed output and the plain source coexist. The next Compress() call must keep the
// compressed file as-is and clean up the stale plain source.
func TestBackupCompressorIdempotentWithStalePlainSource(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	dir, err := os.MkdirTemp("", "backup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	compressor := NewGzipBackupCompressor(dir, processor.GetUncompressedBackupFile, logger)

	plainFileName := "backup.2023-12-18T16:14:00Z.sql"
	plainFilePath := filepath.Join(dir, plainFileName)
	if err := os.WriteFile(plainFilePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if _, err := compressor.Compress(plainFileName); err != nil {
		t.Fatalf("First compress failed: %v", err)
	}

	// Recreate the plain source as if a previous crash had left it behind between Rename and Remove.
	if err := os.WriteFile(plainFilePath, []byte("stale source"), 0644); err != nil {
		t.Fatalf("Failed to recreate stale plain source: %v", err)
	}
	finalPath := filepath.Join(dir, plainFileName+".gz")
	beforeSize, err := fileSize(finalPath)
	if err != nil {
		t.Fatalf("Failed to stat compressed file: %v", err)
	}

	if _, err := compressor.Compress(plainFileName); err != nil {
		t.Fatalf("Idempotent compress failed: %v", err)
	}
	if _, err := os.Stat(plainFilePath); !os.IsNotExist(err) {
		t.Fatalf("Stale plain source should have been removed, got err=%v", err)
	}
	afterSize, err := fileSize(finalPath)
	if err != nil {
		t.Fatalf("Failed to stat compressed file after idempotent compress: %v", err)
	}
	if beforeSize != afterSize {
		t.Fatalf("Compressed file was modified during idempotent compress: %d -> %d", beforeSize, afterSize)
	}
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
