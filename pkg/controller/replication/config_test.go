package replication

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	env "github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"k8s.io/utils/ptr"
)

func TestNewReplicationConfig(t *testing.T) {
	tests := []struct {
		name       string
		env        *env.PodEnvironment
		wantConfig string
		wantErr    bool
	}{
		{
			name: "replication disabled",
			env: &env.PodEnvironment{
				PodName:            "mariadb-0",
				MariadbName:        "mariadb",
				MariaDBReplEnabled: "foo",
			},
			wantErr: true,
		},
		{
			name: "invalid GTID strict mode",
			env: &env.PodEnvironment{
				PodName:                   "mariadb-0",
				MariadbName:               "mariadb",
				MariaDBReplEnabled:        "true",
				MariaDBReplGtidStrictMode: "foo",
			},
			wantErr: true,
		},
		{
			name: "invalid semi-sync enabled",
			env: &env.PodEnvironment{
				PodName:                    "mariadb-0",
				MariadbName:                "mariadb",
				MariaDBReplEnabled:         "true",
				MariaDBReplSemiSyncEnabled: "foo",
			},
			wantErr: true,
		},
		{
			name: "invalid semi-sync master timeout",
			env: &env.PodEnvironment{
				PodName:                          "mariadb-0",
				MariadbName:                      "mariadb",
				MariaDBReplEnabled:               "true",
				MariaDBReplSemiSyncMasterTimeout: "foo",
			},
			wantErr: true,
		},
		{
			name: "invalid server ID",
			env: &env.PodEnvironment{
				PodName:            "foo",
				MariadbName:        "mariadb",
				MariaDBReplEnabled: "true",
			},
			wantErr: true,
		},
		{
			name: "invalid master sync binlog",
			env: &env.PodEnvironment{
				PodName:                     "mariadb-0",
				MariadbName:                 "mariadb",
				MariaDBReplEnabled:          "true",
				MariaDBReplMasterSyncBinlog: "foo",
			},
			wantErr: true,
		},
		{
			name: "minimal replication enabled",
			env: &env.PodEnvironment{
				PodName:            "mariadb-0",
				MariadbName:        "mariadb",
				MariaDBReplEnabled: "true",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
server_id=10
`,
			wantErr: false,
		},
		{
			name: "minimal semi-sync replication enabled",
			env: &env.PodEnvironment{
				PodName:                    "mariadb-0",
				MariadbName:                "mariadb",
				MariaDBReplEnabled:         "true",
				MariaDBReplSemiSyncEnabled: "true",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
server_id=10
`,
			wantErr: false,
		},
		{
			name: "missing semi-sync master timeout",
			env: &env.PodEnvironment{
				PodName:                            "mariadb-0",
				MariadbName:                        "mariadb",
				MariaDBReplEnabled:                 "true",
				MariaDBReplGtidStrictMode:          "true",
				MariaDBReplSemiSyncEnabled:         "true",
				MariaDBReplSemiSyncMasterWaitPoint: "AFTER_SYNC",
				MariaDBReplMasterSyncBinlog:        "1",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
gtid_strict_mode
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
rpl_semi_sync_master_wait_point=AFTER_SYNC
server_id=10
sync_binlog=1
`,
			wantErr: false,
		},
		{
			name: "missing semi-sync master wait point",
			env: &env.PodEnvironment{
				PodName:                          "mariadb-0",
				MariadbName:                      "mariadb",
				MariaDBReplEnabled:               "true",
				MariaDBReplGtidStrictMode:        "true",
				MariaDBReplSemiSyncEnabled:       "true",
				MariaDBReplSemiSyncMasterTimeout: "5000",
				MariaDBReplMasterSyncBinlog:      "1",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
gtid_strict_mode
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
rpl_semi_sync_master_timeout=5000
server_id=10
sync_binlog=1
`,
			wantErr: false,
		},
		{
			name: "with custom GTID domain ID",
			env: &env.PodEnvironment{
				PodName:                 "mariadb-0",
				MariadbName:             "mariadb",
				MariaDBReplEnabled:      "true",
				MariaDBReplGtidDomainID: "1",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
gtid_domain_id=1
server_id=10
`,
			wantErr: false,
		},
		{
			name: "with custom server ID start index",
			env: &env.PodEnvironment{
				PodName:                       "mariadb-2",
				MariadbName:                   "mariadb",
				MariaDBReplEnabled:            "true",
				MariaDBReplServerIDStartIndex: "100",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
server_id=102
`,
			wantErr: false,
		},
		{
			name: "all values present",
			env: &env.PodEnvironment{
				PodName:                            "mariadb-0",
				MariadbName:                        "mariadb",
				MariaDBReplEnabled:                 "true",
				MariaDBReplGtidStrictMode:          "true",
				MariaDBReplGtidDomainID:            "1",
				MariaDBReplServerIDStartIndex:      "100",
				MariaDBReplSemiSyncEnabled:         "true",
				MariaDBReplSemiSyncMasterTimeout:   "5000",
				MariaDBReplSemiSyncMasterWaitPoint: "AFTER_SYNC",
				MariaDBReplMasterSyncBinlog:        "1",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
gtid_strict_mode
gtid_domain_id=1
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
rpl_semi_sync_master_timeout=5000
rpl_semi_sync_master_wait_point=AFTER_SYNC
server_id=100
sync_binlog=1
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewReplicationConfig(tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare as normalized strings (avoids surprises with newlines/whitespace)
			got := strings.TrimSpace(string(config))
			want := strings.TrimSpace(tt.wantConfig)

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("unexpected config (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGtidDomainID(t *testing.T) {
	tests := []struct {
		name          string
		rawGtidDomain string
		want          *int
		wantErr       bool
	}{
		{
			name:          "empty string returns nil",
			rawGtidDomain: "",
			want:          nil,
			wantErr:       false,
		},
		{
			name:          "valid GTID domain ID zero",
			rawGtidDomain: "0",
			want:          ptr.To(0),
			wantErr:       false,
		},
		{
			name:          "valid GTID domain ID",
			rawGtidDomain: "42",
			want:          ptr.To(42),
			wantErr:       false,
		},
		{
			name:          "valid GTID domain ID large",
			rawGtidDomain: "999999",
			want:          ptr.To(999999),
			wantErr:       false,
		},
		{
			name:          "invalid string",
			rawGtidDomain: "foo",
			want:          nil,
			wantErr:       true,
		},
		{
			name:          "invalid float",
			rawGtidDomain: "3.14",
			want:          nil,
			wantErr:       true,
		},
		{
			name:          "invalid with whitespace",
			rawGtidDomain: " 42 ",
			want:          nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := gtidDomainID(tt.rawGtidDomain)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.want == nil && got != nil {
				t.Fatalf("expected nil, got %v", *got)
			}
			if tt.want != nil && got == nil {
				t.Fatalf("expected %v, got nil", *tt.want)
			}
			if tt.want != nil && got != nil && *tt.want != *got {
				t.Errorf("expected %v, got %v", *tt.want, *got)
			}
		})
	}
}

func TestServerIDStartIndex(t *testing.T) {
	tests := []struct {
		name          string
		rawStartIndex string
		want          int
		wantErr       bool
	}{
		{
			name:          "empty string returns default",
			rawStartIndex: "",
			want:          10,
			wantErr:       false,
		},
		{
			name:          "valid start index zero",
			rawStartIndex: "0",
			want:          0,
			wantErr:       false,
		},
		{
			name:          "valid start index",
			rawStartIndex: "100",
			want:          100,
			wantErr:       false,
		},
		{
			name:          "valid start index large",
			rawStartIndex: "999999",
			want:          999999,
			wantErr:       false,
		},
		{
			name:          "invalid string",
			rawStartIndex: "foo",
			want:          0,
			wantErr:       true,
		},
		{
			name:          "invalid float",
			rawStartIndex: "3.14",
			want:          0,
			wantErr:       true,
		},
		{
			name:          "invalid with whitespace",
			rawStartIndex: " 10 ",
			want:          0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := serverIDStartIndex(tt.rawStartIndex)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestServerID(t *testing.T) {
	tests := []struct {
		name       string
		podName    string
		startIndex int
		want       int
		wantErr    bool
	}{
		{
			name:       "first pod with default start index",
			podName:    "mariadb-0",
			startIndex: 10,
			want:       10,
			wantErr:    false,
		},
		{
			name:       "second pod with default start index",
			podName:    "mariadb-1",
			startIndex: 10,
			want:       11,
			wantErr:    false,
		},
		{
			name:       "third pod with default start index",
			podName:    "mariadb-2",
			startIndex: 10,
			want:       12,
			wantErr:    false,
		},
		{
			name:       "first pod with custom start index",
			podName:    "mariadb-0",
			startIndex: 100,
			want:       100,
			wantErr:    false,
		},
		{
			name:       "second pod with custom start index",
			podName:    "mariadb-1",
			startIndex: 100,
			want:       101,
			wantErr:    false,
		},
		{
			name:       "pod with zero start index",
			podName:    "mariadb-5",
			startIndex: 0,
			want:       5,
			wantErr:    false,
		},
		{
			name:       "pod with large index",
			podName:    "mariadb-99",
			startIndex: 10,
			want:       109,
			wantErr:    false,
		},
		{
			name:       "invalid pod name",
			podName:    "foo",
			startIndex: 10,
			want:       0,
			wantErr:    true,
		},
		{
			name:       "pod name without number",
			podName:    "mariadb",
			startIndex: 10,
			want:       0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := serverId(tt.podName, tt.startIndex)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}
