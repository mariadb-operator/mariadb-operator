package sql

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("buildChangeMasterQuery", func() {
	DescribeTable("builds the CHANGE MASTER query",
		func(options []ChangeMasterOpt, wantQuery string, wantErr bool) {
			query, err := buildChangeMasterQuery(options...)

			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())

			Expect(query).To(Equal(wantQuery))
		},
		Entry("missing host",
			[]ChangeMasterOpt{
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
			},
			"",
			true,
		),
		Entry("missing credentials",
			[]ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
			},
			"",
			true,
		),
		Entry("valid without SSL",
			[]ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterGtid("CurrentPos"),
			},
			`CHANGE MASTER  TO
MASTER_HOST='127.0.0.1',
MASTER_PORT=3306,
MASTER_USER='repl',
MASTER_PASSWORD='password',
MASTER_USE_GTID=CurrentPos;
`,
			false,
		),
		Entry("missing SSL paths",
			[]ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterSSL("", "", ""),
			},
			"",
			true,
		),
		Entry("valid with SSL",
			[]ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterGtid("CurrentPos"),
				WithChangeMasterSSL("/etc/pki/client.crt", "/etc/pki/client.key", "/etc/pki/ca.crt"),
			},
			`CHANGE MASTER  TO
MASTER_SSL=1,
MASTER_SSL_CERT='/etc/pki/client.crt',
MASTER_SSL_KEY='/etc/pki/client.key',
MASTER_SSL_CA='/etc/pki/ca.crt',
MASTER_SSL_VERIFY_SERVER_CERT=1,
MASTER_HOST='127.0.0.1',
MASTER_PORT=3306,
MASTER_USER='repl',
MASTER_PASSWORD='password',
MASTER_USE_GTID=CurrentPos;
`,
			false,
		),
		Entry("valid with Retries",
			[]ChangeMasterOpt{
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterGtid("CurrentPos"),
				WithChangeMasterRetries(10),
			},
			`CHANGE MASTER  TO
MASTER_HOST='127.0.0.1',
MASTER_PORT=3306,
MASTER_USER='repl',
MASTER_PASSWORD='password',
MASTER_CONNECT_RETRY=10,
MASTER_USE_GTID=CurrentPos;
`,
			false,
		),
		Entry("valid with custom connection",
			[]ChangeMasterOpt{
				WithChangeMasterConnectionName("replica"),
				WithChangeMasterHost("127.0.0.1"),
				WithChangeMasterPort(3306),
				WithChangeMasterCredentials("repl", "password"),
				WithChangeMasterGtid("CurrentPos"),
			},
			`CHANGE MASTER 'replica' TO
MASTER_HOST='127.0.0.1',
MASTER_PORT=3306,
MASTER_USER='repl',
MASTER_PASSWORD='password',
MASTER_USE_GTID=CurrentPos;
`,
			false,
		),
	)
})

var _ = Describe("requireQuery", func() {
	DescribeTable("builds the REQUIRE query",
		func(require *mariadbv1alpha1.TLSRequirements, wantQuery string, wantErr bool) {
			gotQuery, err := requireQuery(require)

			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(gotQuery).To(Equal(wantQuery))
		},
		Entry("nil", (*mariadbv1alpha1.TLSRequirements)(nil), "", true),
		Entry("empty", &mariadbv1alpha1.TLSRequirements{}, "", true),
		Entry("SSL",
			&mariadbv1alpha1.TLSRequirements{
				SSL: ptr.To(true),
			},
			"REQUIRE SSL",
			false,
		),
		Entry("X509",
			&mariadbv1alpha1.TLSRequirements{
				X509: ptr.To(true),
			},
			"REQUIRE X509",
			false,
		),
		Entry("Issuer",
			&mariadbv1alpha1.TLSRequirements{
				Issuer: ptr.To("/CN=mariadb-galera-ca"),
			},
			"REQUIRE ISSUER '/CN=mariadb-galera-ca'",
			false,
		),
		Entry("Subject",
			&mariadbv1alpha1.TLSRequirements{
				Subject: ptr.To("/CN=mariadb-galera-client"),
			},
			"REQUIRE SUBJECT '/CN=mariadb-galera-client'",
			false,
		),
		Entry("Issuer and Subject",
			&mariadbv1alpha1.TLSRequirements{
				Issuer:  ptr.To("/CN=mariadb-galera-ca"),
				Subject: ptr.To("/CN=mariadb-galera-client"),
			},
			"REQUIRE ISSUER '/CN=mariadb-galera-ca' AND SUBJECT '/CN=mariadb-galera-client'",
			false,
		),
		Entry("Multiple",
			&mariadbv1alpha1.TLSRequirements{
				SSL:     ptr.To(true),
				X509:    ptr.To(true),
				Issuer:  ptr.To("/CN=mariadb-galera-ca"),
				Subject: ptr.To("/CN=mariadb-galera-client"),
			},
			"",
			true,
		),
	)
})
