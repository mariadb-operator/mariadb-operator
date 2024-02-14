package recovery

import (
	"reflect"
	"testing"
)

func TestGaleraStateMarshal(t *testing.T) {
	tests := []struct {
		name        string
		galeraState *GaleraState
		want        string
		wantErr     bool
	}{
		{
			name: "invalid uuid",
			galeraState: &GaleraState{
				Version:         "2.1",
				UUID:            "foo",
				Seqno:           1,
				SafeToBootstrap: false,
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "safe_to_bootstrap false",
			galeraState: &GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: false,
			},
			want: `version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 0`,
			wantErr: false,
		},
		{
			name: "safe_to_bootstrap true",
			galeraState: &GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			want: `version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 1`,
			wantErr: false,
		},
		{
			name: "negative seqno",
			galeraState: &GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           -1,
				SafeToBootstrap: false,
			},
			want: `version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: -1
safe_to_bootstrap: 0`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := tt.galeraState.Marshal()
			if tt.wantErr && err == nil {
				t.Fatal("error expected, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error unexpected, got %v", err)
			}
			if tt.want != string(bytes) {
				t.Fatalf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.want, string(bytes))
			}
		})
	}
}

func TestGaleraStateUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    GaleraState
		wantErr bool
	}{
		{
			name: "empty",
			bytes: []byte(`
`),
			want:    GaleraState{},
			wantErr: true,
		},
		{
			name: "comment",
			bytes: []byte(`# GALERA saved state
version: 2.1
uuid:    05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   1
safe_to_bootstrap: 1`),
			want: GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			wantErr: false,
		},
		{
			name: "indentation",
			bytes: []byte(`# GALERA saved state
version: 												2.1
uuid:  05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   																				1
safe_to_bootstrap: 			1`),
			want: GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			wantErr: false,
		},
		{
			name: "invalid uuid",
			bytes: []byte(`# GALERA saved state
version: 2.1
uuid:    foo
seqno:   -1
safe_to_bootstrap: 1`),
			want:    GaleraState{},
			wantErr: true,
		},
		{
			name: "invalid seqno",
			bytes: []byte(`# GALERA saved state
version: 2.1
uuid:    05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   foo
safe_to_bootstrap: 1`),
			want:    GaleraState{},
			wantErr: true,
		},
		{
			name: "invalid safe_to_bootstrap",
			bytes: []byte(`# GALERA saved state
version: 2.1
uuid:    05f061bd-02a3-11ee-857c-aa370ff6666b
seqno:   1
safe_to_bootstrap: true`),
			want:    GaleraState{},
			wantErr: true,
		},
		{
			name: "safe_to_bootstrap true",
			bytes: []byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 1`),
			want: GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: true,
			},
			wantErr: false,
		},
		{
			name: "safe_to_bootstrap false",
			bytes: []byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: 1
safe_to_bootstrap: 0`),
			want: GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           1,
				SafeToBootstrap: false,
			},
			wantErr: false,
		},
		{
			name: "negative seqno",
			bytes: []byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
seqno: -1
safe_to_bootstrap: 0`),
			want: GaleraState{
				Version:         "2.1",
				UUID:            "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno:           -1,
				SafeToBootstrap: false,
			},
			wantErr: false,
		},
		{
			name: "missing safe_to_bootstrap",
			bytes: []byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
safe_to_bootstrap: 0`),
			want:    GaleraState{},
			wantErr: true,
		},
		{
			name: "missing seqno",
			bytes: []byte(`version: 2.1
uuid: 05f061bd-02a3-11ee-857c-aa370ff6666b
safe_to_bootstrap: 0`),
			want:    GaleraState{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var galeraState GaleraState
			err := galeraState.Unmarshal(tt.bytes)
			if tt.wantErr && err == nil {
				t.Fatal("error expected, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error unexpected, got %v", err)
			}
			if !reflect.DeepEqual(tt.want, galeraState) {
				t.Fatalf("unexpected result:\nexpected:\n%v\ngot:\n%v\n", tt.want, galeraState)
			}
		})
	}
}

func TestBootstrapValidate(t *testing.T) {
	tests := []struct {
		name      string
		bootstrap Bootstrap
		wantErr   bool
	}{
		{
			name: "invalid uuid",
			bootstrap: Bootstrap{
				UUID:  "foo",
				Seqno: 1,
			},
			wantErr: true,
		},
		{
			name: "seqno",
			bootstrap: Bootstrap{
				UUID:  "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno: 1,
			},
			wantErr: false,
		},
		{
			name: "negative seqno",
			bootstrap: Bootstrap{
				UUID:  "05f061bd-02a3-11ee-857c-aa370ff6666b",
				Seqno: -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bootstrap.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("error expected, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error unexpected, got %v", err)
			}
		})
	}
}

func TestBootstrapUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    Bootstrap
		wantErr bool
	}{
		{
			name: "empty",
			bytes: []byte(`
`),
			want:    Bootstrap{},
			wantErr: true,
		},
		{
			name: "missig position",
			//nolint
			bytes: []byte(`2023-06-04  8:24:23 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 86033
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
			want:    Bootstrap{},
			wantErr: true,
		},
		{
			name:    "invalid uuid",
			bytes:   []byte(`2023-06-04  8:24:23 0 [Note] WSREP: Recovered position: foo:1`),
			want:    Bootstrap{},
			wantErr: true,
		},
		{
			name:    "invalid seqno",
			bytes:   []byte(`2023-06-04  8:24:23 0 [Note] WSREP: Recovered position: 15d9a0ef-02b1-11ee-9499-decd8e34642e:bar`),
			want:    Bootstrap{},
			wantErr: true,
		},
		{
			name: "single position",
			//nolint
			bytes: []byte(`2023-06-04  8:24:23 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 86033
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
			want: Bootstrap{
				UUID:  "15d9a0ef-02b1-11ee-9499-decd8e34642e",
				Seqno: 1,
			},
			wantErr: false,
		},
		{
			name: "multiple positions",
			//nolint
			bytes: []byte(`2023-06-04  8:24:16 0 [Note] Starting MariaDB 10.11.3-MariaDB-1:10.11.3+maria~ubu2204 source revision 0bb31039f54bd6a0dc8f0fc7d40e6b58a51998b0 as process 84826
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
			want: Bootstrap{
				UUID:  "08dd3b99-ac6b-46f8-84bd-8cb8f9f949b0",
				Seqno: 3,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bootstrap Bootstrap
			err := bootstrap.Unmarshal(tt.bytes)
			if tt.wantErr && err == nil {
				t.Fatal("error expected, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error unexpected, got %v", err)
			}
			if !reflect.DeepEqual(tt.want, bootstrap) {
				t.Fatalf("unexpected result:\nexpected:\n%v\ngot:\n%v\n", tt.want, bootstrap)
			}
		})
	}
}
