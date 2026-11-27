package binlog

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestGetObjectName(t *testing.T) {
	meta := &BinlogMetadata{
		ServerId:       10,
		BinlogFilename: "mariadb-repl-bin.000001",
		ObjectName:     "server-10/mariadb-repl-bin.000001.1772362800",
	}
	legacyMeta := &BinlogMetadata{
		ServerId:       10,
		BinlogFilename: "mariadb-repl-bin.000001",
	}

	tests := []struct {
		name        string
		meta        *BinlogMetadata
		compression mariadbv1alpha1.CompressAlgorithm
		want        string
		wantErr     bool
	}{
		{
			name: "no compression",
			meta: meta,
			want: "server-10/mariadb-repl-bin.000001.1772362800",
		},
		{
			name:        "none",
			meta:        meta,
			compression: mariadbv1alpha1.CompressNone,
			want:        "server-10/mariadb-repl-bin.000001.1772362800",
		},
		{
			name:        "gzip",
			meta:        meta,
			compression: mariadbv1alpha1.CompressGzip,
			want:        "server-10/mariadb-repl-bin.000001.1772362800.gz",
		},
		{
			name:        "bzip2",
			meta:        meta,
			compression: mariadbv1alpha1.CompressBzip2,
			want:        "server-10/mariadb-repl-bin.000001.1772362800.bz2",
		},
		{
			name:        "zstd",
			meta:        meta,
			compression: mariadbv1alpha1.CompressZstd,
			want:        "server-10/mariadb-repl-bin.000001.1772362800.zst",
		},
		{
			name:        "legacy gzip",
			meta:        legacyMeta,
			compression: mariadbv1alpha1.CompressGzip,
			want:        "server-10/mariadb-repl-bin.000001.gz",
		},
		{
			name:        "invalid compression",
			meta:        meta,
			compression: mariadbv1alpha1.CompressAlgorithm("foo"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pitr := &mariadbv1alpha1.PointInTimeRecovery{
				Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
					Compression: tt.compression,
				},
			}
			objectName, err := getObjectName(tt.meta, pitr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, objectName)
		})
	}
}
