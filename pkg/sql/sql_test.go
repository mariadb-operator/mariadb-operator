package sql

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBuildChangeMasterQuery(t *testing.T) {
	tests := []struct {
		name      string
		options   []ChangeMasterOpt
		wantQuery string
		wantErr   bool
	}{
		{
			name: "missing host",
			options: []ChangeMasterOpt{
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
			},
			wantQuery: "",
			wantErr:   true,
		},
		{
			name: "missing credentials",
			options: []ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
			},
			wantQuery: "",
			wantErr:   true,
		},
		{
			name: "valid without SSL",
			options: []ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterGtid("CurrentPos"),
			},
			wantQuery: `CHANGE MASTER 'mariadb-operator' TO
MASTER_HOST='127.0.0.1',
MASTER_PORT=3306,
MASTER_USER='repl',
MASTER_PASSWORD='password',
MASTER_USE_GTID=CurrentPos,
MASTER_CONNECT_RETRY=10;
`,
			wantErr: false,
		},
		{
			name: "missing SSL paths",
			options: []ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterSSL("", "", ""),
			},
			wantQuery: "",
			wantErr:   true,
		},
		{
			name: "valid with SSL",
			options: []ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterGtid("CurrentPos"),
				WithChangeMasterSSL("/etc/pki/client.crt", "/etc/pki/client.key", "/etc/pki/ca.crt"),
			},
			wantQuery: `CHANGE MASTER 'mariadb-operator' TO
MASTER_HOST='127.0.0.1',
MASTER_PORT=3306,
MASTER_USER='repl',
MASTER_PASSWORD='password',
MASTER_USE_GTID=CurrentPos,
MASTER_CONNECT_RETRY=10,
MASTER_SSL=1,
MASTER_SSL_CERT='/etc/pki/client.crt',
MASTER_SSL_KEY='/etc/pki/client.key',
MASTER_SSL_CA='/etc/pki/ca.crt',
MASTER_SSL_VERIFY_SERVER_CERT=1;
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := buildChangeMasterQuery(tt.options...)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(query, tt.wantQuery); diff != "" {
				t.Errorf("unexpected query (-want +got):\n%s", diff)
			}
		})
	}
}
