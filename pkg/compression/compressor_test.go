package compression

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/dsnet/compress/bzip2"
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
			fileName:        "backup.2023-12-18T16:14:00Z.gzip.sql",
		},
		{
			name:            "bzip2",
			newCompressorFn: NewBzip2BackupCompressor,
			fileName:        "backup.2023-12-18T16:14:00Z.bzip2.sql",
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

func TestDecompressStream(t *testing.T) {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	tests := []struct {
		name            string
		newCompressorFn func(basePath string, getUncompressedFilename GetUncompressedFilenameFn, logger logr.Logger) Compressor
		compressData    func(data []byte) ([]byte, error)
	}{
		{
			name:            "nop",
			newCompressorFn: NewNopCompressor,
			compressData: func(data []byte) ([]byte, error) {
				return data, nil
			},
		},
		{
			name:            "gzip",
			newCompressorFn: NewGzipBackupCompressor,
			compressData: func(data []byte) ([]byte, error) {
				var buf bytes.Buffer
				writer := gzip.NewWriter(&buf)
				if _, err := writer.Write(data); err != nil {
					return nil, err
				}
				if err := writer.Close(); err != nil {
					return nil, err
				}
				return buf.Bytes(), nil
			},
		},
		{
			name:            "bzip2",
			newCompressorFn: NewBzip2BackupCompressor,
			compressData: func(data []byte) ([]byte, error) {
				var buf bytes.Buffer
				writer, err := bzip2.NewWriter(&buf, &bzip2.WriterConfig{Level: bzip2.DefaultCompression})
				if err != nil {
					return nil, err
				}
				if _, err := writer.Write(data); err != nil {
					return nil, err
				}
				if err := writer.Close(); err != nil {
					return nil, err
				}
				return buf.Bytes(), nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "backup_stream_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(dir)

			compressor := tt.newCompressorFn(dir, processor.GetUncompressedBackupFile, logger)

			streamingCompressor, ok := compressor.(StreamingCompressor)
			if !ok {
				t.Fatalf("Compressor does not implement StreamingCompressor interface")
			}

			compressedData, err := tt.compressData([]byte(content))
			if err != nil {
				t.Fatalf("Failed to compress test data: %v", err)
			}

			src := bytes.NewReader(compressedData)
			var dst bytes.Buffer

			bytesWritten, err := streamingCompressor.DecompressStream(&dst, src)
			if err != nil {
				t.Fatalf("Failed to decompress stream: %v", err)
			}

			if bytesWritten != int64(len(content)) {
				t.Errorf("Bytes written mismatch: got %d, want %d", bytesWritten, len(content))
			}

			if dst.String() != content {
				t.Errorf("Decompressed content does not match original content:\nGot: %s\nWant: %s", dst.String(), content)
			}
		})
	}
}
