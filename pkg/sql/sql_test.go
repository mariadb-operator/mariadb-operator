package sql

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/utils/ptr"
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

func TestRequireQuery(t *testing.T) {
	tests := []struct {
		name      string
		require   *mariadbv1alpha1.TLSRequirements
		wantQuery string
		wantErr   bool
	}{
		{
			name:      "nil",
			require:   nil,
			wantQuery: "",
			wantErr:   true,
		},
		{
			name:      "empty",
			require:   &mariadbv1alpha1.TLSRequirements{},
			wantQuery: "",
			wantErr:   true,
		},
		{
			name: "SSL",
			require: &mariadbv1alpha1.TLSRequirements{
				SSL: ptr.To(true),
			},
			wantQuery: "REQUIRE SSL",
			wantErr:   false,
		},
		{
			name: "X509",
			require: &mariadbv1alpha1.TLSRequirements{
				X509: ptr.To(true),
			},
			wantQuery: "REQUIRE X509",
			wantErr:   false,
		},
		{
			name: "Issuer",
			require: &mariadbv1alpha1.TLSRequirements{
				Issuer: ptr.To("/CN=mariadb-galera-ca"),
			},
			wantQuery: "REQUIRE ISSUER '/CN=mariadb-galera-ca'",
			wantErr:   false,
		},
		{
			name: "Subject",
			require: &mariadbv1alpha1.TLSRequirements{
				Subject: ptr.To("/CN=mariadb-galera-client"),
			},
			wantQuery: "REQUIRE SUBJECT '/CN=mariadb-galera-client'",
			wantErr:   false,
		},
		{
			name: "Issuer and Subject",
			require: &mariadbv1alpha1.TLSRequirements{
				Issuer:  ptr.To("/CN=mariadb-galera-ca"),
				Subject: ptr.To("/CN=mariadb-galera-client"),
			},
			wantQuery: "REQUIRE ISSUER '/CN=mariadb-galera-ca' AND SUBJECT '/CN=mariadb-galera-client'",
			wantErr:   false,
		},
		{
			name: "Multiple",
			require: &mariadbv1alpha1.TLSRequirements{
				SSL:     ptr.To(true),
				X509:    ptr.To(true),
				Issuer:  ptr.To("/CN=mariadb-galera-ca"),
				Subject: ptr.To("/CN=mariadb-galera-client"),
			},
			wantQuery: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, err := requireQuery(tt.require)

			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
			if diff := cmp.Diff(tt.wantQuery, gotQuery); diff != "" {
				t.Errorf("unexpected bundle content (-want +got):\n%s", diff)
			}
		})
	}
}
