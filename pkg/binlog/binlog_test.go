package binlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	mariadbrepl "github.com/mariadb-operator/mariadb-operator/v25/pkg/replication"
	"sigs.k8s.io/yaml"
)

func TestBinlogPath(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name      string
		indexFile *BinlogIndex
		fromGtid  *mariadbrepl.Gtid
		untilTime time.Time
		wantPath  []string
		wantErr   bool
	}{
		{
			name:      "single GTID binlog in server-10",
			indexFile: mustParseTestFile(t, "single-gtid-binlog-server-10.yaml"),
			fromGtid:  mustParseGtid(t, "0-10-1"),
			untilTime: time.Now(),
			wantPath: []string{
				"server-10/mariadb-repl-bin.000002",
			},
			wantErr: false,
		},
		{
			name:      "multiple GTID binlog in server-10",
			indexFile: mustParseTestFile(t, "multiple-gtid-binlog-server-10.yaml"),
			fromGtid:  mustParseGtid(t, "0-10-1"),
			untilTime: time.Now(),
			wantPath: []string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
				"server-10/mariadb-repl-bin.000007",
				"server-10/mariadb-repl-bin.000008",
				"server-10/mariadb-repl-bin.000009",
				"server-10/mariadb-repl-bin.000010",
				"server-10/mariadb-repl-bin.000011",
			},
			wantErr: false,
		},
		{
			name:      "filter by server-10 gtid and date",
			indexFile: mustParseTestFile(t, "failover-1205-1208.yaml"),
			fromGtid:  mustParseGtid(t, "0-10-40"),
			untilTime: mustParseDate(t, "2026-02-04T12:05:00Z"),
			wantPath: []string{
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
			},
			wantErr: false,
		},
		{
			name:      "filter by server-11 gtid and date",
			indexFile: mustParseTestFile(t, "failover-1205-1208.yaml"),
			fromGtid:  mustParseGtid(t, "0-11-100"),
			untilTime: mustParseDate(t, "2026-02-04T12:06:50Z"),
			wantPath: []string{
				"server-11/mariadb-repl-bin.000002",
				"server-11/mariadb-repl-bin.000003",
				"server-11/mariadb-repl-bin.000004",
				"server-11/mariadb-repl-bin.000005",
				"server-11/mariadb-repl-bin.000006",
			},
			wantErr: false,
		},
		{
			name:      "failover at 12:05 from server-10 to server-10",
			indexFile: mustParseTestFile(t, "failover-1205-1208.yaml"),
			fromGtid:  mustParseGtid(t, "0-10-1"),
			untilTime: mustParseDate(t, "2026-02-04T12:06:30Z"),
			wantPath: []string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
				// FAILOVER to server-11
				"server-11/mariadb-repl-bin.000001",
				"server-11/mariadb-repl-bin.000002",
				"server-11/mariadb-repl-bin.000003",
				"server-11/mariadb-repl-bin.000004",
				"server-11/mariadb-repl-bin.000005",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binlogMetas, err := tt.indexFile.BinlogPath(tt.fromGtid, tt.untilTime, logger)
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
			if diff := cmp.Diff(getBinlogPath(binlogMetas), tt.wantPath); diff != "" {
				t.Errorf("unexpected binlog path (-want +got):\n%s", diff)
			}
		})
	}
}

func mustParseTestFile(t *testing.T, file string) *BinlogIndex {
	t.Helper()
	testFile := filepath.Join("test", file)
	bytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to parse test file %s: %v", testFile, err)
	}
	var bi BinlogIndex
	if err := yaml.Unmarshal(bytes, &bi); err != nil {
		t.Fatalf("failed to unmarshal test file %s: %v", testFile, err)
	}
	return &bi
}

func mustParseGtid(t *testing.T, s string) *mariadbrepl.Gtid {
	t.Helper()
	g, err := mariadbrepl.ParseGtid(s)
	if err != nil {
		t.Fatalf("failed to parse gtid %s: %v", s, err)
	}
	return g
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("failed to parse date %s: %v", s, err)
	}
	return d
}

func getBinlogPath(binlogMetas []BinlogMetadata) []string {
	path := make([]string, len(binlogMetas))
	for i, binlogMeta := range binlogMetas {
		path[i] = binlogMeta.ObjectStoragePath()
	}
	return path
}
