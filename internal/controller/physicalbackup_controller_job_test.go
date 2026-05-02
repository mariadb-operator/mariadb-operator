package controller

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

// TestGetBackupFileName_DoesNotAppendCompressionExtension documents the bug:
//
//	getBackupFileName previously appended the compression extension (e.g. ".gz")
//	to the dump-output filename, producing names like "physicalbackup-<ts>.xb.gz".
//	mariadb-backup then wrote the *uncompressed* xbstream content to that file
//	(name implies compressed but content is not). The operator-backup container's
//	compressFile step (pkg/compression/backup_compressor.go) then appended the
//	extension again — yielding "physicalbackup-<ts>.xb.gz.gz".
//
//	A doubly-extended name is rejected by PhysicalBackupProcessor.IsValidBackupFile
//	(pkg/backup/processor.go::ParseCompressionAlgorithm expects 2 or 3 dot-parts),
//	so the file became unreachable for any restore Job — wedging replica recovery
//	completely.
//
//	Fix: emit the plain ".xb" name regardless of compression. The compression
//	step appends the extension. This mirrors the equivalent fix for logical
//	backups in pkg/command/backup.go::newBackupFile.
func TestGetBackupFileName_DoesNotAppendCompressionExtension(t *testing.T) {
	now := time.Date(2026, 5, 2, 18, 53, 16, 0, time.UTC)

	tests := []struct {
		name        string
		compression mariadbv1alpha1.CompressAlgorithm
		want        string
	}{
		{name: "no compression", compression: mariadbv1alpha1.CompressNone, want: "physicalbackup-20260502185316.xb"},
		{name: "gzip", compression: mariadbv1alpha1.CompressGzip, want: "physicalbackup-20260502185316.xb"},
		{name: "bzip2", compression: mariadbv1alpha1.CompressBzip2, want: "physicalbackup-20260502185316.xb"},
		{name: "empty (defaults to none)", compression: "", want: "physicalbackup-20260502185316.xb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := &mariadbv1alpha1.PhysicalBackup{
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Compression: tt.compression,
				},
			}
			got, err := getBackupFileName(backup, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("getBackupFileName(%v) = %q, want %q (must NOT include compression extension — that is appended later by compressFile)",
					tt.compression, got, tt.want)
			}
		})
	}
}
