package binlog

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/compression"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestBinlogNumberingRestarted(t *testing.T) {
	dataDir := t.TempDir()
	mustWriteTestBinlog(t, filepath.Join(dataDir, "mariadb-repl-bin.000001"), 1772362800)
	mustWriteTestBinlog(t, filepath.Join(dataDir, "mariadb-repl-bin.000002"), 1772362860)
	archiver := &Archiver{dataDir: dataDir, logger: logr.Discard()}

	tests := []struct {
		name    string
		binlogs []string
		status  mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus
		want    bool
	}{
		{
			name:    "nothing archived",
			binlogs: []string{"mariadb-repl-bin.000001", "mariadb-repl-bin.000002"},
			status:  mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{},
			want:    false,
		},
		{
			name:    "same generation",
			binlogs: []string{"mariadb-repl-bin.000001", "mariadb-repl-bin.000002"},
			status: mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{
				LastArchivedBinaryLog: "mariadb-repl-bin.000002",
				LastArchivedTime:      metav1.NewTime(time.Unix(1772362900, 0)),
			},
			want: false,
		},
		{
			name:    "restart past the last archived number",
			binlogs: []string{"mariadb-repl-bin.000001", "mariadb-repl-bin.000002"},
			status: mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{
				LastArchivedBinaryLog: "mariadb-repl-bin.000001",
				LastArchivedTime:      metav1.NewTime(time.Unix(1772362700, 0)),
			},
			want: true,
		},
		{
			name:    "no last archived time",
			binlogs: []string{"mariadb-repl-bin.000001", "mariadb-repl-bin.000002"},
			status: mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{
				LastArchivedBinaryLog: "mariadb-repl-bin.000001",
			},
			want: false,
		},
		{
			name:    "restart below the last archived number",
			binlogs: []string{"mariadb-repl-bin.000001", "mariadb-repl-bin.000002"},
			status: mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{
				LastArchivedBinaryLog: "mariadb-repl-bin.000005",
				LastArchivedTime:      metav1.NewTime(time.Unix(1772362700, 0)),
			},
			want: true,
		},
		{
			name:    "last archived binlog purged",
			binlogs: []string{"mariadb-repl-bin.000006", "mariadb-repl-bin.000007"},
			status: mariadbv1alpha1.MariaDBPointInTimeRecoveryStatus{
				LastArchivedBinaryLog: "mariadb-repl-bin.000005",
				LastArchivedTime:      metav1.NewTime(time.Unix(1772362700, 0)),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restarted, err := archiver.binlogNumberingRestarted(tt.binlogs, &tt.status)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, restarted)
		})
	}
}

func TestUpdateBinlogIndex(t *testing.T) {
	dataDir := t.TempDir()
	mustWriteTestBinlog(t, filepath.Join(dataDir, "mariadb-repl-bin.000001"), 1772362800)
	archiver := &Archiver{dataDir: dataDir, logger: logr.Discard()}
	mdb := &mariadbv1alpha1.MariaDB{}
	pitr := &mariadbv1alpha1.PointInTimeRecovery{
		Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
			Compression: mariadbv1alpha1.CompressGzip,
		},
	}
	compressor, err := compression.NewCompressor(mariadbv1alpha1.CompressGzip, 0)
	assert.NoError(t, err)
	binlogs := []string{"mariadb-repl-bin.000001"}
	objectName := "server-10/mariadb-repl-bin.000001.1772362800.gz"

	tests := []struct {
		name    string
		objects []string
	}{
		{
			name:    "archived binlog",
			objects: []string{objectName},
		},
		{
			name:    "binlog missing in storage",
			objects: []string{"server-10/mariadb-repl-bin.000001.gz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := newFakeBlobStorage(tt.objects...)
			uploader := NewUploader(dataDir, storage, compressor, logr.Discard())

			index, err := archiver.updateBinlogIndex(context.Background(), binlogs, 10, mdb, pitr, storage, uploader)
			assert.NoError(t, err)
			assert.True(t, index.Exists(10, "mariadb-repl-bin.000001", metav1.NewTime(time.Unix(1772362800, 0))))
			assert.Contains(t, storage.objects, objectName)

			var stored BinlogIndex
			assert.NoError(t, yaml.Unmarshal(storage.objects[BinlogIndexName], &stored))
			assert.Equal(t, index.Binlogs, stored.Binlogs)
		})
	}
}

type fakeBlobStorage struct {
	objects map[string][]byte
}

func newFakeBlobStorage(objects ...string) *fakeBlobStorage {
	storage := &fakeBlobStorage{objects: map[string][]byte{}}
	for _, object := range objects {
		storage.objects[object] = []byte{}
	}
	return storage
}

func (f *fakeBlobStorage) PutObjectWithOptions(_ context.Context, fileName string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.objects[fileName] = data
	return nil
}

func (f *fakeBlobStorage) GetObjectWithOptions(_ context.Context, fileName string) (io.ReadCloser, error) {
	data, ok := f.objects[fileName]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeBlobStorage) Exists(_ context.Context, fileName string) (bool, error) {
	_, ok := f.objects[fileName]
	return ok, nil
}

func (f *fakeBlobStorage) FPutObjectWithOptions(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *fakeBlobStorage) FGetObjectWithOptions(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *fakeBlobStorage) RemoveWithOptions(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *fakeBlobStorage) PrefixedFileName(fileName string) string {
	return fileName
}

func (f *fakeBlobStorage) UnprefixedFilename(fileName string) string {
	return fileName
}

func (f *fakeBlobStorage) GetPrefix() string {
	return ""
}

func (f *fakeBlobStorage) ListObjectsWithOptions(context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeBlobStorage) IsAuthenticated(context.Context) bool {
	return true
}

func (f *fakeBlobStorage) IsNotFound(error) bool {
	return false
}
