package replication

import (
	"strings"

	env "github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("NewReplicationConfig", func() {
	DescribeTable("builds the replication config",
		func(podEnv *env.PodEnvironment, wantConfig string, wantErr bool) {
			config, err := NewReplicationConfig(podEnv)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

			// Compare as normalized strings (avoids surprises with newlines/whitespace)
			got := strings.TrimSpace(string(config))
			want := strings.TrimSpace(wantConfig)

			Expect(got).To(Equal(want))
		},
		Entry("replication disabled",
			&env.PodEnvironment{
				PodName:            "mariadb-0",
				MariadbName:        "mariadb",
				MariaDBReplEnabled: "foo",
			},
			"",
			true,
		),
		Entry("invalid GTID strict mode",
			&env.PodEnvironment{
				PodName:                   "mariadb-0",
				MariadbName:               "mariadb",
				MariaDBReplEnabled:        "true",
				MariaDBReplGtidStrictMode: "foo",
			},
			"",
			true,
		),
		Entry("invalid semi-sync enabled",
			&env.PodEnvironment{
				PodName:                    "mariadb-0",
				MariadbName:                "mariadb",
				MariaDBReplEnabled:         "true",
				MariaDBReplSemiSyncEnabled: "foo",
			},
			"",
			true,
		),
		Entry("invalid semi-sync master timeout",
			&env.PodEnvironment{
				PodName:                          "mariadb-0",
				MariadbName:                      "mariadb",
				MariaDBReplEnabled:               "true",
				MariaDBReplSemiSyncMasterTimeout: "foo",
			},
			"",
			true,
		),
		Entry("invalid server ID",
			&env.PodEnvironment{
				PodName:            "foo",
				MariadbName:        "mariadb",
				MariaDBReplEnabled: "true",
			},
			"",
			true,
		),
		Entry("invalid master sync binlog",
			&env.PodEnvironment{
				PodName:                     "mariadb-0",
				MariadbName:                 "mariadb",
				MariaDBReplEnabled:          "true",
				MariaDBReplMasterSyncBinlog: "foo",
			},
			"",
			true,
		),
		Entry("minimal replication enabled",
			&env.PodEnvironment{
				PodName:            "mariadb-0",
				MariadbName:        "mariadb",
				MariaDBReplEnabled: "true",
			},
			`[mariadb]
log_bin
log_basename=mariadb
server_id=10
`,
			false,
		),
		Entry("minimal semi-sync replication enabled",
			&env.PodEnvironment{
				PodName:                    "mariadb-0",
				MariadbName:                "mariadb",
				MariaDBReplEnabled:         "true",
				MariaDBReplSemiSyncEnabled: "true",
			},
			`[mariadb]
log_bin
log_basename=mariadb
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
server_id=10
`,
			false,
		),
		Entry("missing semi-sync master timeout",
			&env.PodEnvironment{
				PodName:                            "mariadb-0",
				MariadbName:                        "mariadb",
				MariaDBReplEnabled:                 "true",
				MariaDBReplGtidStrictMode:          "true",
				MariaDBReplSemiSyncEnabled:         "true",
				MariaDBReplSemiSyncMasterWaitPoint: "AFTER_SYNC",
				MariaDBReplMasterSyncBinlog:        "1",
			},
			`[mariadb]
log_bin
log_basename=mariadb
gtid_strict_mode
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
rpl_semi_sync_master_wait_point=AFTER_SYNC
server_id=10
sync_binlog=1
`,
			false,
		),
		Entry("missing semi-sync master wait point",
			&env.PodEnvironment{
				PodName:                          "mariadb-0",
				MariadbName:                      "mariadb",
				MariaDBReplEnabled:               "true",
				MariaDBReplGtidStrictMode:        "true",
				MariaDBReplSemiSyncEnabled:       "true",
				MariaDBReplSemiSyncMasterTimeout: "5000",
				MariaDBReplMasterSyncBinlog:      "1",
			},
			`[mariadb]
log_bin
log_basename=mariadb
gtid_strict_mode
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_slave_enabled=ON
rpl_semi_sync_master_timeout=5000
server_id=10
sync_binlog=1
`,
			false,
		),
		Entry("with custom GTID domain ID",
			&env.PodEnvironment{
				PodName:                 "mariadb-0",
				MariadbName:             "mariadb",
				MariaDBReplEnabled:      "true",
				MariaDBReplGtidDomainID: "1",
			},
			`[mariadb]
log_bin
log_basename=mariadb
gtid_domain_id=1
server_id=10
`,
			false,
		),
		Entry("with custom server ID start index",
			&env.PodEnvironment{
				PodName:                       "mariadb-2",
				MariadbName:                   "mariadb",
				MariaDBReplEnabled:            "true",
				MariaDBReplServerIDStartIndex: "100",
			},
			`[mariadb]
log_bin
log_basename=mariadb
server_id=102
`,
			false,
		),
		Entry("all values present",
			&env.PodEnvironment{
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
			`[mariadb]
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
			false,
		),
	)
})

var _ = Describe("GtidDomainID", func() {
	DescribeTable("parses the GTID domain ID",
		func(rawGtidDomain string, want *int, wantErr bool) {
			got, err := gtidDomainID(rawGtidDomain)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

			Expect(got).To(Equal(want))
		},
		Entry("empty string returns nil",
			"",
			nil,
			false,
		),
		Entry("valid GTID domain ID zero",
			"0",
			ptr.To(0),
			false,
		),
		Entry("valid GTID domain ID",
			"42",
			ptr.To(42),
			false,
		),
		Entry("valid GTID domain ID large",
			"999999",
			ptr.To(999999),
			false,
		),
		Entry("invalid string",
			"foo",
			nil,
			true,
		),
		Entry("invalid float",
			"3.14",
			nil,
			true,
		),
		Entry("invalid with whitespace",
			" 42 ",
			nil,
			true,
		),
	)
})

var _ = Describe("ServerIDStartIndex", func() {
	DescribeTable("parses the server ID start index",
		func(rawStartIndex string, want int, wantErr bool) {
			got, err := serverIDStartIndex(rawStartIndex)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

			Expect(got).To(Equal(want))
		},
		Entry("empty string returns default",
			"",
			10,
			false,
		),
		Entry("valid start index zero",
			"0",
			0,
			false,
		),
		Entry("valid start index",
			"100",
			100,
			false,
		),
		Entry("valid start index large",
			"999999",
			999999,
			false,
		),
		Entry("invalid string",
			"foo",
			0,
			true,
		),
		Entry("invalid float",
			"3.14",
			0,
			true,
		),
		Entry("invalid with whitespace",
			" 10 ",
			0,
			true,
		),
	)
})

var _ = Describe("ServerID", func() {
	DescribeTable("computes the server ID",
		func(podName string, startIndex int, want int, wantErr bool) {
			got, err := serverId(podName, startIndex)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

			Expect(got).To(Equal(want))
		},
		Entry("first pod with default start index",
			"mariadb-0",
			10,
			10,
			false,
		),
		Entry("second pod with default start index",
			"mariadb-1",
			10,
			11,
			false,
		),
		Entry("third pod with default start index",
			"mariadb-2",
			10,
			12,
			false,
		),
		Entry("first pod with custom start index",
			"mariadb-0",
			100,
			100,
			false,
		),
		Entry("second pod with custom start index",
			"mariadb-1",
			100,
			101,
			false,
		),
		Entry("pod with zero start index",
			"mariadb-5",
			0,
			5,
			false,
		),
		Entry("pod with large index",
			"mariadb-99",
			10,
			109,
			false,
		),
		Entry("invalid pod name",
			"foo",
			10,
			0,
			true,
		),
		Entry("pod name without number",
			"mariadb",
			10,
			0,
			true,
		),
	)
})
