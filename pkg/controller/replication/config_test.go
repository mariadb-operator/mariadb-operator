package replication

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	env "github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
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
			name: "all values present",
			env: &env.PodEnvironment{
				PodName:                            "mariadb-0",
				MariadbName:                        "mariadb",
				MariaDBReplEnabled:                 "true",
				MariaDBReplGtidStrictMode:          "true",
				MariaDBReplSemiSyncEnabled:         "true",
				MariaDBReplSemiSyncMasterTimeout:   "5000",
				MariaDBReplSemiSyncMasterWaitPoint: "AFTER_SYNC",
				MariaDBReplMasterSyncBinlog:        "1",
			},
			wantConfig: `[mariadb]
log_bin
log_basename=mariadb
gtid_strict_mode
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
rpl_semi_sync_master_timeout=5000
rpl_semi_sync_master_wait_point=AFTER_SYNC
server_id=10
sync_binlog=1
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
