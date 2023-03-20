package replication

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrimaryCnf(t *testing.T) {
	tests := []struct {
		name        string
		replication mariadbv1alpha1.Replication
		wantConfig  string
	}{
		{
			name: "Async replication",
			replication: mariadbv1alpha1.Replication{
				Mode: mariadbv1alpha1.ReplicationModeAsync,
			},
			wantConfig: `[mariadb]
log-bin
log-basename=mariadb
`,
		},
		{
			name: "SemiSync replication",
			replication: mariadbv1alpha1.Replication{
				Mode: mariadbv1alpha1.ReplicationModeSemiSync,
			},
			wantConfig: `[mariadb]
log-bin
log-basename=mariadb
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_master_timeout=30000
`,
		},
		{
			name: "SemiSync replication",
			replication: mariadbv1alpha1.Replication{
				Mode: mariadbv1alpha1.ReplicationModeSemiSync,
				PrimaryTimeout: func() *metav1.Duration {
					d, err := time.ParseDuration("10s")
					if err != nil {
						t.Fatalf("unexpected error parsing duration: %v", err)
					}
					return &metav1.Duration{Duration: d}
				}(),
				WaitPoint: func() *mariadbv1alpha1.WaitPoint {
					wp := mariadbv1alpha1.WaitPointAfterSync
					return &wp
				}(),
			},
			wantConfig: `[mariadb]
log-bin
log-basename=mariadb
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_master_timeout=10000
rpl_semi_sync_master_wait_point=AFTER_SYNC
`,
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			config, err := PrimaryCnf(tt.replication)
			if err != nil {
				t.Fatalf("unexpected error generating primary.cnf: %v", err)
			}
			if config != tt.wantConfig {
				t.Errorf("unpexpected configuration, \ngot:\n%v\nexpected:\n%v\n", config, tt.wantConfig)
			}
		})
	}
}

func TestPrimarySql(t *testing.T) {
	sqlOpts := PrimarySqlOpts{
		ReplUser:     "repl",
		ReplPassword: "secret",
		Users: []PrimarySqlUser{
			{
				Username: "foo",
				Password: "super-secret",
			},
			{
				Username: "bar",
				Password: "even-more-secret",
			},
		},
		Databases: []string{"foo", "bar"},
	}
	config, err := PrimarySql(sqlOpts)
	if err != nil {
		t.Fatalf("unexpected error generating primary.sql: %v", err)
	}
	wantConfig := `CREATE USER 'repl'@'%' IDENTIFIED BY 'secret';
GRANT REPLICATION REPLICA ON *.* TO 'repl'@'%';
CREATE USER 'foo'@'%' IDENTIFIED BY 'super-secret';
GRANT ALL PRIVILEGES ON *.* TO 'foo'@'%';
CREATE USER 'bar'@'%' IDENTIFIED BY 'even-more-secret';
GRANT ALL PRIVILEGES ON *.* TO 'bar'@'%';
CREATE DATABASE foo;
CREATE DATABASE bar;
`

	if config != wantConfig {
		t.Errorf("unpexpected configuration, \ngot:\n%v\nexpected:\n%v\n", config, wantConfig)
	}
}

func TestReplicaCnf(t *testing.T) {
	tests := []struct {
		name        string
		replication mariadbv1alpha1.Replication
		wantConfig  string
	}{
		{
			name: "Async replication",
			replication: mariadbv1alpha1.Replication{
				Mode: mariadbv1alpha1.ReplicationModeAsync,
			},
			wantConfig: `[mariadb]
read_only=1
log-basename=mariadb
`,
		},
		{
			name: "SemiSync replication",
			replication: mariadbv1alpha1.Replication{
				Mode: mariadbv1alpha1.ReplicationModeSemiSync,
			},
			wantConfig: `[mariadb]
read_only=1
log-basename=mariadb
rpl_semi_sync_slave_enabled=ON
`,
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			config, err := ReplicaCnf(tt.replication)
			if err != nil {
				t.Fatalf("unexpected error generating replica.cnf: %v", err)
			}
			if config != tt.wantConfig {
				t.Errorf("unpexpected configuration, \ngot:\n%v\nexpected:\n%v\n", config, tt.wantConfig)
			}
		})
	}
}

func TestReplicaSql(t *testing.T) {
	tests := []struct {
		name       string
		opts       ReplicaSqlOpts
		wantConfig string
	}{
		{
			name: "Replica SQL",
			opts: ReplicaSqlOpts{
				Meta: metav1.ObjectMeta{
					Name:      "mariadb",
					Namespace: "default",
				},
				User:     "repl",
				Password: "super-secret",
			},
			wantConfig: `CHANGE MASTER TO 
MASTER_HOST='mariadb-0.mariadb.default.svc.cluster.local',
MASTER_USER='repl',
MASTER_PASSWORD='super-secret',
MASTER_CONNECT_RETRY=10;
`,
		},
		{
			name: "Replica SQL with retries",
			opts: ReplicaSqlOpts{
				Meta: metav1.ObjectMeta{
					Name:      "mariadb",
					Namespace: "default",
				},
				User:     "repl",
				Password: "super-secret",
				Retries:  func() *int32 { r := int32(5); return &r }(),
			},
			wantConfig: `CHANGE MASTER TO 
MASTER_HOST='mariadb-0.mariadb.default.svc.cluster.local',
MASTER_USER='repl',
MASTER_PASSWORD='super-secret',
MASTER_CONNECT_RETRY=5;
`,
		},
	}
	for _, tt := range tests {
		config, err := ReplicaSql(tt.opts)
		if err != nil {
			t.Fatalf("unexpected error generating replica.sql: %v", err)
		}
		if config != tt.wantConfig {
			t.Errorf("unpexpected configuration, \ngot:\n%v\nexpected:\n%v\n", config, tt.wantConfig)
		}
	}
}

func TestInitSh(t *testing.T) {
	opts := InitShOpts{
		PrimaryCnf: "foo.cnf",
		PrimarySql: "foo.sql",
		ReplicaCnf: "bar.cnf",
		ReplicaSql: "bar.sql",
	}
	config, err := InitSh(opts)
	if err != nil {
		t.Fatalf("unexpected error generating init.sh: %v", err)
	}
	wantConfig := `#!/bin/bash

set -euo pipefail

echo '🦭 Staring init-repl';
echo '🔧 /mnt/mysql'
ls /mnt/mysql
echo '🔧 /mnt/repl'
ls /mnt/repl

if [[ $(find  /mnt/mysql -type f -name '*.cnf') ]]; then
	echo '🔧 Syncing *.cnf files provided by user'
	cp /mnt/mysql/*.cnf /etc/mysql/conf.d
fi

[[ $HOSTNAME =~ -([0-9]+)$ ]] || exit 1
ordinal=${BASH_REMATCH[1]}

if [[ $ordinal -eq 0 ]]; then
	echo '🦭 Syncing primary config files'
	cp /mnt/repl/foo.cnf /etc/mysql/conf.d/server-id.cnf
	cp /mnt/repl/foo.sql /docker-entrypoint-initdb.d
else
	echo '🪞 Syncing replica config files'
	cp /mnt/repl/bar.cnf /etc/mysql/conf.d/server-id.cnf
	cp /mnt/repl/bar.sql /docker-entrypoint-initdb.d
fi

echo server-id=$((1000 + $ordinal)) >> /etc/mysql/conf.d/server-id.cnf
echo '🔧 /etc/mysql/conf.d/'
ls /etc/mysql/conf.d/
echo '🔧 /etc/mysql/conf.d/server-id.cnf'
cat /etc/mysql/conf.d/server-id.cnf
`

	if config != wantConfig {
		t.Errorf("unpexpected configuration, \ngot:\n%v\nexpected:\n%v\n", config, wantConfig)
	}
}
