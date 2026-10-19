package compression

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
)

func requirePigz(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pigz"); err != nil {
		t.Skip("pigz must be installed to run this test")
	}
}

func TestPigzCompressor(t *testing.T) {
	requirePigz(t)
	content := bytes.Repeat([]byte("INSERT INTO test VALUES (1, 'backup data');\n"), 32768)
	for _, threads := range []int{0, 1, 4} {
		t.Run(strconv.Itoa(threads), func(t *testing.T) {
			compressor, err := NewCompressor(mariadbv1alpha1.CompressPigz, threads)
			if err != nil {
				t.Fatal(err)
			}
			gzipCompressor := &GzipCompressor{}
			// Both directions must be compatible with the existing gzip implementation.
			for _, pair := range [][2]Compressor{{compressor, gzipCompressor}, {gzipCompressor, compressor}} {
				var compressed, restored bytes.Buffer
				if err := pair[0].Compress(context.Background(), &compressed, bytes.NewReader(content)); err != nil {
					t.Fatal(err)
				}
				if err := pair[1].Decompress(context.Background(), &restored, &compressed); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(restored.Bytes(), content) {
					t.Fatal("restored data differs from original")
				}
			}
		})
	}
}

func TestPigzArguments(t *testing.T) {
	for _, threads := range []int{0, 4} {
		t.Run(strconv.Itoa(threads), func(t *testing.T) {
			dir := t.TempDir()
			wantArgs := "--stdout"
			if threads > 0 {
				wantArgs += " --processes " + strconv.Itoa(threads)
			}
			// Check that the child receives the configured threads and no inherited options.
			script := "#!/bin/sh\n[ \"$*\" = \"" + wantArgs + "\" ] && [ -z \"$GZIP$PIGZ\" ]\n"
			if err := os.WriteFile(filepath.Join(dir, "pigz"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			t.Setenv("GZIP", "--zlib")
			t.Setenv("PIGZ", "--zip --processes 1")
			compressor := &PigzCompressor{concurrency: threads}
			if err := compressor.Compress(context.Background(), io.Discard, strings.NewReader("")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPigzErrors(t *testing.T) {
	t.Run("missing executable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		err := (&PigzCompressor{}).Compress(context.Background(), io.Discard, strings.NewReader("data"))
		if !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("expected missing executable error, got %v", err)
		}
	})
	t.Run("invalid threads", func(t *testing.T) {
		err := (&PigzCompressor{concurrency: -1}).Compress(context.Background(), io.Discard, strings.NewReader("data"))
		if err == nil {
			t.Fatal("expected negative thread count to fail")
		}
	})
	t.Run("canceled context", func(t *testing.T) {
		requirePigz(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := (&PigzCompressor{}).Compress(ctx, io.Discard, strings.NewReader("data"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation error, got %v", err)
		}
	})
	t.Run("corrupt input", func(t *testing.T) {
		requirePigz(t)
		err := (&PigzCompressor{}).Decompress(context.Background(), io.Discard, strings.NewReader("not gzip data"))
		if err == nil {
			t.Fatal("expected corrupt compressed data to fail")
		}
	})
}

func TestPigzBackupRestore(t *testing.T) {
	requirePigz(t)
	tests := []struct {
		name      string
		fileName  string
		processor backup.BackupProcessor
	}{
		{"logical", "backup.2023-12-18T16:14:00Z.sql.gz", backup.NewLogicalBackupProcessor()},
		{"physical", "physicalbackup-20231218161400.xb.gz", backup.NewPhysicalBackupProcessor()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, tt.fileName)
			content := []byte("backup data")
			if err := os.WriteFile(filePath, content, 0644); err != nil {
				t.Fatal(err)
			}
			compressor, err := NewBackupCompressor(mariadbv1alpha1.CompressPigz, 2, dir,
				tt.processor.GetUncompressedBackupFile, logr.Discard())
			if err != nil {
				t.Fatal(err)
			}
			if err := compressor.Compress(tt.fileName); err != nil {
				t.Fatal(err)
			}
			// Restore infers gzip from the filename, rather than the Backup's compression setting.
			algorithm, err := tt.processor.ParseCompressionAlgorithm(tt.fileName)
			if err != nil {
				t.Fatal(err)
			}
			restorer, err := NewBackupCompressor(algorithm, 0, dir, tt.processor.GetUncompressedBackupFile, logr.Discard())
			if err != nil {
				t.Fatal(err)
			}
			restoredPath, err := restorer.Decompress(tt.fileName)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := os.ReadFile(restoredPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(restored, content) {
				t.Fatal("restored backup differs from original")
			}
		})
	}
}

func TestPigzBackupCompressionFailure(t *testing.T) {
	dir := t.TempDir()
	fileName := "backup.2023-12-18T16:14:00Z.sql.gz"
	filePath := filepath.Join(dir, fileName)
	content := []byte("original backup")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	compressor := NewPigzBackupCompressor(2, dir, backup.NewLogicalBackupProcessor().GetUncompressedBackupFile, logr.Discard())
	if err := compressor.Compress(fileName); err == nil {
		t.Fatal("expected compression without pigz to fail")
	}
	original, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, content) {
		t.Fatal("failed compression modified the original backup")
	}
	if _, err := os.Stat(filePath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file was not removed: %v", err)
	}
}
