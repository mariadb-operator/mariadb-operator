package recovery

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GaleraState Marshal", func() {
	DescribeTable("marshaling a GaleraState",
		func(galeraState *GaleraState, want string, wantErr bool) {
			bytes, err := galeraState.Marshal()
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(string(bytes)).To(Equal(want))
		},
		Entry("invalid uuid",
			&GaleraState{
				Version:         "2.1",
				UUID:            "foo",
				Seqno:           1,
				SafeToBootstrap: false,
			},
			"",
			true,
		),
		Entry("safe_to_bootstrap false",
			&GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: false,
			},
			`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 0`,
			false,
		),
		Entry("safe_to_bootstrap true",
			&GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 1`,
			false,
		),
		Entry("negative seqno",
			&GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           -1,
				SafeToBootstrap: false,
			},
			`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: -1
safe_to_bootstrap: 0`,
			false,
		),
	)
})

var _ = Describe("GaleraState Unmarshal", func() {
	DescribeTable("unmarshaling a GaleraState",
		func(b []byte, want GaleraState, wantErr bool) {
			var galeraState GaleraState
			err := galeraState.Unmarshal(b)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(galeraState).To(Equal(want))
		},
		Entry("empty",
			[]byte(`
`),
			GaleraState{},
			true,
		),
		Entry("comment",
			[]byte(`# GALERA saved state
version: 2.1
uuid:    05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   1
safe_to_bootstrap: 1`),
			GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			false,
		),
		Entry("indentation",
			[]byte(`# GALERA saved state
version: 												2.1
uuid:  05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   																				1
safe_to_bootstrap: 			1`),
			GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			false,
		),
		Entry("invalid uuid",
			[]byte(`# GALERA saved state
version: 2.1
uuid:    foo
seqno:   -1
safe_to_bootstrap: 1`),
			GaleraState{},
			true,
		),
		Entry("invalid seqno",
			[]byte(`# GALERA saved state
version: 2.1
uuid:    05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   foo
safe_to_bootstrap: 1`),
			GaleraState{},
			true,
		),
		Entry("invalid safe_to_bootstrap",
			[]byte(`# GALERA saved state
version: 2.1
uuid:    05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   1
safe_to_bootstrap: true`),
			GaleraState{},
			true,
		),
		Entry("safe_to_bootstrap true",
			[]byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 1`),
			GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			false,
		),
		Entry("safe_to_bootstrap false",
			[]byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 0`),
			GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: false,
			},
			false,
		),
		Entry("negative seqno",
			[]byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: -1
safe_to_bootstrap: 0`),
			GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           -1,
				SafeToBootstrap: false,
			},
			false,
		),
		Entry("missing safe_to_bootstrap",
			[]byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
safe_to_bootstrap: 0`),
			GaleraState{},
			true,
		),
		Entry("missing seqno",
			[]byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
safe_to_bootstrap: 0`),
			GaleraState{},
			true,
		),
	)
})

var _ = Describe("Bootstrap Validate", func() {
	DescribeTable("validating a Bootstrap",
		func(bootstrap Bootstrap, wantErr bool) {
			err := bootstrap.Validate()
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("invalid uuid",
			Bootstrap{
				UUID:  "foo",
				Seqno: 1,
			},
			true,
		),
		Entry("seqno",
			Bootstrap{
				UUID:  "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno: 1,
			},
			false,
		),
		Entry("negative seqno",
			Bootstrap{
				UUID:  "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno: -1,
			},
			false,
		),
	)
})

var _ = Describe("Bootstrap Unmarshal", func() {
	logger := logr.Discard()

	DescribeTable("unmarshaling a Bootstrap",
		func(b []byte, want Bootstrap, wantErr bool) {
			var bootstrap Bootstrap
			err := bootstrap.Unmarshal(b, logger)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(bootstrap).To(Equal(want))
		},
		Entry("empty",
			[]byte(`
`),
			Bootstrap{},
			true,
		),
		Entry("missig position",
			//nolint
			[]byte(`2023-06-04  8:24:23 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 86033
2023-06-04  8:24:23 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2023-06-04  8:24:23 0 [Note] InnoDB: Number of transaction pools: 1
2023-06-04  8:24:23 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2023-06-04  8:24:23 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2023-06-04  8:24:23 0 [Note] InnoDB: Using liburing
2023-06-04  8:24:23 0 [Note] InnoDB: Initializing buffer pool, total size = 128.000MiB, chunk size = 2.000MiB
2023-06-04  8:24:23 0 [Note] InnoDB: Completed initialization of buffer pool
2023-06-04  8:24:23 0 [Note] InnoDB: File system buffers for log disabled (block size=512 bytes)
2023-06-04  8:24:23 0 [Note] InnoDB: 128 rollback segments are active.
`),
			Bootstrap{},
			true,
		),
		Entry("invalid uuid",
			[]byte(`2023-06-04  8:24:23 0 [Note] WSREP: Recovered position: foo:1`),
			Bootstrap{},
			true,
		),
		Entry("invalid seqno",
			[]byte(`2023-06-04  8:24:23 0 [Note] WSREP: Recovered position: 15d9a0ef-02b1-11ee-9499-decd8e34642e:bar`),
			Bootstrap{},
			true,
		),
		Entry("single position",
			//nolint
			[]byte(`2023-06-04  8:24:23 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 86033
2023-06-04  8:24:23 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2023-06-04  8:24:23 0 [Note] InnoDB: Number of transaction pools: 1
2023-06-04  8:24:23 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2023-06-04  8:24:23 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2023-06-04  8:24:23 0 [Note] InnoDB: Using liburing
2023-06-04  8:24:23 0 [Note] InnoDB: Initializing buffer pool, total size = 128.000MiB, chunk size = 2.000MiB
2023-06-04  8:24:23 0 [Note] InnoDB: Completed initialization of buffer pool
2023-06-04  8:24:23 0 [Note] InnoDB: File system buffers for log disabled (block size=512 bytes)
2023-06-04  8:24:23 0 [Note] InnoDB: 128 rollback segments are active.
2023-06-04  8:24:23 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2023-06-04  8:24:23 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2023-06-04  8:24:23 0 [Note] InnoDB: log sequence number 54530; transaction id 30
2023-06-04  8:24:23 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2023-06-04  8:24:23 0 [Note] Plugin 'FEEDBACK' is disabled.
2023-06-04  8:24:23 0 [Note] Server socket created on IP: '0.0.0.0'.
2023-06-04  8:24:23 0 [Note] WSREP: Recovered position: 15d9a0ef-02b1-11ee-9499-decd8e34642e:1
Warning: Memory not freed: 280
`),
			Bootstrap{
				UUID:  "15d9a0ef-02b1-11ee-9499-decd8e34642e",
				Seqno: 1,
			},
			false,
		),
		Entry("1 single position with GTID",
			//nolint
			[]byte(`2025-10-14 10:14:25 0 [Note] slave_connections_needed_for_purge changed to 0 because of Galera. Change it to 1 or higher if this Galera node is also Master in a normal replication setup
2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
2025-10-14 10:14:25 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2025-10-14 10:14:25 0 [Note] InnoDB: Number of transaction pools: 1
2025-10-14 10:14:25 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2025-10-14 10:14:25 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2025-10-14 10:14:25 0 [Note] InnoDB: Using Linux native AIO
2025-10-14 10:14:25 0 [Note] InnoDB: Initializing buffer pool, total size = 1.000GiB, chunk size = 16.000MiB
2025-10-14 10:14:25 0 [Note] InnoDB: Completed initialization of buffer pool
2025-10-14 10:14:25 0 [Note] InnoDB: Buffered log writes (block size=512 bytes)
2025-10-14 10:14:25 0 [Note] InnoDB: End of log at LSN=2734997
2025-10-14 10:14:26 0 [Note] InnoDB: Opened 3 undo tablespaces
2025-10-14 10:14:26 0 [Note] InnoDB: 128 rollback segments in 3 undo tablespaces are active.
2025-10-14 10:14:26 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2025-10-14 10:14:26 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2025-10-14 10:14:26 0 [Note] InnoDB: log sequence number 2734997; transaction id 63
2025-10-14 10:14:26 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2025-10-14 10:14:26 0 [Note] Plugin 'FEEDBACK' is disabled.
2025-10-14 10:14:26 server_audit: MariaDB Audit Plugin version 2.5.0 STARTED.
2025-10-14 10:14:26 server_audit: Query cache is enabled with the TABLE events. Some table reads can be veiled.
2025-10-14 10:14:26 server_audit: logging started to the file /var/lib/mysql/server_audit.log.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '0.0.0.0'.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '::'.
2025-10-14 10:14:26 0 [Note] WSREP: Recovered position: 808ffebd-a7f3-11f0-b6b4-b3b81fce961a:48451,201-2001-48431
2025-10-14 10:14:26 server_audit: STOPPED
`),
			Bootstrap{
				UUID:  "808ffebd-a7f3-11f0-b6b4-b3b81fce961a",
				Seqno: 48451,
			},
			false,
		),
		Entry("2 single position with GTID",
			//nolint
			[]byte(`2025-10-14 10:14:25 0 [Note] slave_connections_needed_for_purge changed to 0 because of Galera. Change it to 1 or higher if this Galera node is also Master in a normal replication setup
2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
2025-10-14 10:14:25 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2025-10-14 10:14:25 0 [Note] InnoDB: Number of transaction pools: 1
2025-10-14 10:14:25 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2025-10-14 10:14:25 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2025-10-14 10:14:25 0 [Note] InnoDB: Using Linux native AIO
2025-10-14 10:14:25 0 [Note] InnoDB: Initializing buffer pool, total size = 1.000GiB, chunk size = 16.000MiB
2025-10-14 10:14:25 0 [Note] InnoDB: Completed initialization of buffer pool
2025-10-14 10:14:25 0 [Note] InnoDB: Buffered log writes (block size=512 bytes)
2025-10-14 10:14:25 0 [Note] InnoDB: End of log at LSN=2734997
2025-10-14 10:14:26 0 [Note] InnoDB: Opened 3 undo tablespaces
2025-10-14 10:14:26 0 [Note] InnoDB: 128 rollback segments in 3 undo tablespaces are active.
2025-10-14 10:14:26 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2025-10-14 10:14:26 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2025-10-14 10:14:26 0 [Note] InnoDB: log sequence number 2734997; transaction id 63
2025-10-14 10:14:26 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2025-10-14 10:14:26 0 [Note] Plugin 'FEEDBACK' is disabled.
2025-10-14 10:14:26 server_audit: MariaDB Audit Plugin version 2.5.0 STARTED.
2025-10-14 10:14:26 server_audit: Query cache is enabled with the TABLE events. Some table reads can be veiled.
2025-10-14 10:14:26 server_audit: logging started to the file /var/lib/mysql/server_audit.log.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '0.0.0.0'.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '::'.
2025-10-14 10:14:26 0 [Note] WSREP: Recovered position: 808ffebd-a7f3-11f0-b6b4-b3b81fce961a:201-2001-48431,48451
2025-10-14 10:14:26 server_audit: STOPPED
`),
			Bootstrap{
				UUID:  "808ffebd-a7f3-11f0-b6b4-b3b81fce961a",
				Seqno: 48451,
			},
			false,
		),
		Entry("multiple positions",
			//nolint
			[]byte(`2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
2023-06-04  8:24:16 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2023-06-04  8:24:16 0 [Note] InnoDB: Number of transaction pools: 1
2023-06-04  8:24:16 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2023-06-04  8:24:16 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2023-06-04  8:24:16 0 [Note] InnoDB: Using liburing
2023-06-04  8:24:16 0 [Note] InnoDB: Initializing buffer pool, total size = 128.000MiB, chunk size = 2.000MiB
2023-06-04  8:24:16 0 [Note] InnoDB: Completed initialization of buffer pool
2023-06-04  8:24:16 0 [Note] InnoDB: File system buffers for log disabled (block size=512 bytes)
2023-06-04  8:24:16 0 [Note] InnoDB: Starting crash recovery from checkpoint LSN=46590
2023-06-04  8:24:16 0 [Note] InnoDB: Starting final batch to recover 15 pages from redo log.
2023-06-04  8:24:16 0 [Note] InnoDB: 128 rollback segments are active.
2023-06-04  8:24:16 0 [Note] InnoDB: Removed temporary tablespace data file: "./ibtmp1"
2023-06-04  8:24:16 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2023-06-04  8:24:16 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2023-06-04  8:24:16 0 [Note] InnoDB: log sequence number 54202; transaction id 30
2023-06-04  8:24:16 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2023-06-04  8:24:16 0 [Note] Plugin 'FEEDBACK' is disabled.
2023-06-04  8:24:16 0 [Note] Recovering after a crash using tc.log
2023-06-04  8:24:16 0 [Note] Starting table crash recovery...
2023-06-04  8:24:16 0 [Note] Crash table recovery finished.
2023-06-04  8:24:16 0 [Note] Server socket created on IP: '0.0.0.0'.
2023-06-04  8:24:16 0 [Note] WSREP: Recovered position: 15d9a0ef-02b1-11ee-9499-decd8e34642e:1
Warning: Memory not freed: 280
2023-06-04  8:24:17 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 85132
2023-06-04  8:24:17 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2023-06-04  8:24:17 0 [Note] InnoDB: Number of transaction pools: 1
2023-06-04  8:24:17 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2023-06-04  8:24:17 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2023-06-04  8:24:17 0 [Note] InnoDB: Using liburing
2023-06-04  8:24:17 0 [Note] InnoDB: Initializing buffer pool, total size = 128.000MiB, chunk size = 2.000MiB
2023-06-04  8:24:17 0 [Note] InnoDB: Completed initialization of buffer pool
2023-06-04  8:24:17 0 [Note] InnoDB: File system buffers for log disabled (block size=512 bytes)
2023-06-04  8:24:17 0 [Note] InnoDB: 128 rollback segments are active.
2023-06-04  8:24:17 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2023-06-04  8:24:17 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2023-06-04  8:24:17 0 [Note] InnoDB: log sequence number 54530; transaction id 30
2023-06-04  8:24:17 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2023-06-04  8:24:17 0 [Note] Plugin 'FEEDBACK' is disabled.
2023-06-04  8:24:17 0 [Note] Server socket created on IP: '0.0.0.0'.
2023-06-04  8:24:17 0 [Note] WSREP: Recovered position: 0794bb7b-0614-41cd-8301-4fe1d55a1f60:2
Warning: Memory not freed: 280
2023-06-04  8:24:18 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 85425
2023-06-04  8:24:18 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2023-06-04  8:24:18 0 [Note] InnoDB: Number of transaction pools: 1
2023-06-04  8:24:18 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2023-06-04  8:24:18 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2023-06-04  8:24:18 0 [Note] InnoDB: Using liburing
2023-06-04  8:24:18 0 [Note] InnoDB: Initializing buffer pool, total size = 128.000MiB, chunk size = 2.000MiB
2023-06-04  8:24:18 0 [Note] InnoDB: Completed initialization of buffer pool
2023-06-04  8:24:18 0 [Note] InnoDB: File system buffers for log disabled (block size=512 bytes)
2023-06-04  8:24:18 0 [Note] InnoDB: 128 rollback segments are active.
2023-06-04  8:24:18 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2023-06-04  8:24:18 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2023-06-04  8:24:18 0 [Note] InnoDB: log sequence number 54530; transaction id 30
2023-06-04  8:24:18 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2023-06-04  8:24:18 0 [Note] Plugin 'FEEDBACK' is disabled.
2023-06-04  8:24:18 0 [Note] Server socket created on IP: '0.0.0.0'.
2023-06-04  8:24:18 0 [Note] WSREP: Recovered position: 08dd3b99-ac6b-46f8-84bd-8cb8f9f949b0:3
Warning: Memory not freed: 280
`),
			Bootstrap{
				UUID:  "08dd3b99-ac6b-46f8-84bd-8cb8f9f949b0",
				Seqno: 3,
			},
			false,
		),
		Entry("multiple position with GTID",
			//nolint
			[]byte(`2025-10-14 10:14:25 0 [Note] slave_connections_needed_for_purge changed to 0 because of Galera. Change it to 1 or higher if this Galera node is also Master in a normal replication setup
2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
2025-10-14 10:14:25 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2025-10-14 10:14:25 0 [Note] InnoDB: Number of transaction pools: 1
2025-10-14 10:14:25 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2025-10-14 10:14:25 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2025-10-14 10:14:25 0 [Note] InnoDB: Using Linux native AIO
2025-10-14 10:14:25 0 [Note] InnoDB: Initializing buffer pool, total size = 1.000GiB, chunk size = 16.000MiB
2025-10-14 10:14:25 0 [Note] InnoDB: Completed initialization of buffer pool
2025-10-14 10:14:25 0 [Note] InnoDB: Buffered log writes (block size=512 bytes)
2025-10-14 10:14:25 0 [Note] InnoDB: End of log at LSN=2734997
2025-10-14 10:14:26 0 [Note] InnoDB: Opened 3 undo tablespaces
2025-10-14 10:14:26 0 [Note] InnoDB: 128 rollback segments in 3 undo tablespaces are active.
2025-10-14 10:14:26 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2025-10-14 10:14:26 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2025-10-14 10:14:26 0 [Note] InnoDB: log sequence number 2734997; transaction id 63
2025-10-14 10:14:26 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2025-10-14 10:14:26 0 [Note] Plugin 'FEEDBACK' is disabled.
2025-10-14 10:14:26 server_audit: MariaDB Audit Plugin version 2.5.0 STARTED.
2025-10-14 10:14:26 server_audit: Query cache is enabled with the TABLE events. Some table reads can be veiled.
2025-10-14 10:14:26 server_audit: logging started to the file /var/lib/mysql/server_audit.log.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '0.0.0.0'.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '::'.
2025-10-14 10:14:26 0 [Note] WSREP: Recovered position: 808ffebd-a7f3-11f0-b6b4-b3b81fce961a:48451,201-2001-48431
2025-10-14 10:14:26 server_audit: STOPPED
2025-10-14 10:14:25 0 [Note] slave_connections_needed_for_purge changed to 0 because of Galera. Change it to 1 or higher if this Galera node is also Master in a normal replication setup
2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
2025-10-14 10:14:25 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2025-10-14 10:14:25 0 [Note] InnoDB: Number of transaction pools: 1
2025-10-14 10:14:25 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2025-10-14 10:14:25 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2025-10-14 10:14:25 0 [Note] InnoDB: Using Linux native AIO
2025-10-14 10:14:25 0 [Note] InnoDB: Initializing buffer pool, total size = 1.000GiB, chunk size = 16.000MiB
2025-10-14 10:14:25 0 [Note] InnoDB: Completed initialization of buffer pool
2025-10-14 10:14:25 0 [Note] InnoDB: Buffered log writes (block size=512 bytes)
2025-10-14 10:14:25 0 [Note] InnoDB: End of log at LSN=2734997
2025-10-14 10:14:26 0 [Note] InnoDB: Opened 3 undo tablespaces
2025-10-14 10:14:26 0 [Note] InnoDB: 128 rollback segments in 3 undo tablespaces are active.
2025-10-14 10:14:26 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2025-10-14 10:14:26 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2025-10-14 10:14:26 0 [Note] InnoDB: log sequence number 2734997; transaction id 63
2025-10-14 10:14:26 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2025-10-14 10:14:26 0 [Note] Plugin 'FEEDBACK' is disabled.
2025-10-14 10:14:26 server_audit: MariaDB Audit Plugin version 2.5.0 STARTED.
2025-10-14 10:14:26 server_audit: Query cache is enabled with the TABLE events. Some table reads can be veiled.
2025-10-14 10:14:26 server_audit: logging started to the file /var/lib/mysql/server_audit.log.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '0.0.0.0'.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '::'.
2025-10-14 10:14:26 0 [Note] WSREP: Recovered position: 808ffebd-a7f3-11f0-b6b4-b3b81fce961b:48452,201-2001-48431
2025-10-14 10:14:26 server_audit: STOPPED
2025-10-14 10:14:25 0 [Note] slave_connections_needed_for_purge changed to 0 because of Galera. Change it to 1 or higher if this Galera node is also Master in a normal replication setup
2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
2025-10-14 10:14:25 0 [Note] InnoDB: Compressed tables use zlib 1.2.11
2025-10-14 10:14:25 0 [Note] InnoDB: Number of transaction pools: 1
2025-10-14 10:14:25 0 [Note] InnoDB: Using crc32 + pclmulqdq instructions
2025-10-14 10:14:25 0 [Note] mariadbd: O_TMPFILE is not supported on /tmp (disabling future attempts)
2025-10-14 10:14:25 0 [Note] InnoDB: Using Linux native AIO
2025-10-14 10:14:25 0 [Note] InnoDB: Initializing buffer pool, total size = 1.000GiB, chunk size = 16.000MiB
2025-10-14 10:14:25 0 [Note] InnoDB: Completed initialization of buffer pool
2025-10-14 10:14:25 0 [Note] InnoDB: Buffered log writes (block size=512 bytes)
2025-10-14 10:14:25 0 [Note] InnoDB: End of log at LSN=2734997
2025-10-14 10:14:26 0 [Note] InnoDB: Opened 3 undo tablespaces
2025-10-14 10:14:26 0 [Note] InnoDB: 128 rollback segments in 3 undo tablespaces are active.
2025-10-14 10:14:26 0 [Note] InnoDB: Setting file './ibtmp1' size to 12.000MiB. Physically writing the file full; Please wait ...
2025-10-14 10:14:26 0 [Note] InnoDB: File './ibtmp1' size is now 12.000MiB.
2025-10-14 10:14:26 0 [Note] InnoDB: log sequence number 2734997; transaction id 63
2025-10-14 10:14:26 0 [Warning] InnoDB: Skipping buffer pool dump/restore during wsrep recovery.
2025-10-14 10:14:26 0 [Note] Plugin 'FEEDBACK' is disabled.
2025-10-14 10:14:26 server_audit: MariaDB Audit Plugin version 2.5.0 STARTED.
2025-10-14 10:14:26 server_audit: Query cache is enabled with the TABLE events. Some table reads can be veiled.
2025-10-14 10:14:26 server_audit: logging started to the file /var/lib/mysql/server_audit.log.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '0.0.0.0'.
2025-10-14 10:14:26 0 [Note] Server socket created on IP: '::'.
2025-10-14 10:14:26 0 [Note] WSREP: Recovered position: 808ffebd-a7f3-11f0-b6b4-b3b81fce961c:48453,201-2001-48431
2025-10-14 10:14:26 server_audit: STOPPED
`),
			Bootstrap{
				UUID:  "808ffebd-a7f3-11f0-b6b4-b3b81fce961c",
				Seqno: 48453,
			},
			false,
		),
	)
})
