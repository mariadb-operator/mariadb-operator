package compression

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
)

func TestCompressors(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	tests := []struct {
		name            string
		newCompressorFn func(basePath string, getUncompressedFilename GetUncompressedFilenameFn, logger logr.Logger) Compressor
		fileName        string
	}{
		{
			name:            "nop",
			newCompressorFn: NewNopCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql",
		},
		{
			name:            "gzip",
			newCompressorFn: NewGzipBackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql.gz",
		},
		{
			name:            "bzip2",
			newCompressorFn: NewBzip2BackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.sql.bz2",
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

			filePath := filepath.Join(dir, tt.fileName)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			if err := compressor.Compress(filePath); err != nil {
				t.Fatalf("Failed to compress test file: %v", err)
			}
			decompressedFileName, err := compressor.Decompress(filePath)
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
