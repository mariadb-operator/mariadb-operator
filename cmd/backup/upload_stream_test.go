package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	mdbcompression "github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestUploadStreamCommandInheritsGeneratedBackupFlags(t *testing.T) {
	for _, flagName := range []string{
		"max-retention",
		"physical-backup-meta",
		"physical-backup-name",
		"physical-backup-namespace",
		"producer-exit-file",
		"compression-threads",
	} {
		assert.NotNil(t, uploadStreamCommand.Flag(flagName), "expected upload-stream to accept --%s", flagName)
	}
}

func TestUploadCompressedStreamChecksProducerExit(t *testing.T) {
	tests := []struct {
		name         string
		producerExit *string
		wantErr      string
	}{
		{name: "producer succeeded", producerExit: ptr.To("0")},
		{name: "producer failed", producerExit: ptr.To("7"), wantErr: "backup producer exited with code 7"},
		{name: "producer recorded nothing", wantErr: "backup producer did not record an exit code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			exitFile := filepath.Join(dir, "0-backup-producer-exit.txt")
			if tt.producerExit != nil {
				require.NoError(t, os.WriteFile(exitFile, []byte(*tt.producerExit), 0o644))
			}
			storage := backup.NewFileSystemBackupStorage(dir, nil, logr.Discard())
			compressor, err := mdbcompression.NewCompressor(mariadbv1alpha1.CompressGzip, 0)
			require.NoError(t, err)
			source := &producerCheckReader{reader: strings.NewReader("partial-backup"), exitFile: exitFile}

			err = uploadCompressedStream(context.Background(), storage, "physicalbackup-20260101000000.xb.gz",
				source, compressor)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
