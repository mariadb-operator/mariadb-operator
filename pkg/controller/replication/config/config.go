package replication

import (
	"bytes"
	"fmt"
	"html/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ConfigReplicaKey(mariadb *mariadbv1alpha1.MariaDB) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("config-repl-%s", mariadb.Name),
		Namespace: mariadb.Namespace,
	}
}

func PrimaryCnf(replication mariadbv1alpha1.Replication) (string, error) {
	type tplValues struct {
		SemiSync  bool
		Timeout   int64
		WaitPoint string
	}
	tpl := createTpl("primary.cnf", `[mariadb]
log-bin
log-basename=mariadb
{{ if .SemiSync -}}
rpl_semi_sync_master_enabled=ON
rpl_semi_sync_master_timeout={{ or .Timeout 30000 }}
{{ if .WaitPoint -}}
rpl_semi_sync_master_wait_point={{ .WaitPoint }}
{{ end -}}
{{ end -}}
`)

	values := tplValues{
		SemiSync: replication.Mode == mariadbv1alpha1.ReplicationModeSemiSync,
	}
	if replication.PrimaryTimeout != nil {
		values.Timeout = replication.PrimaryTimeout.Milliseconds()
	}
	if replication.WaitPoint != nil {
		val, err := replication.WaitPoint.MariaDBFormat()
		if err != nil {
			return "", fmt.Errorf("error generating primary.cnf: %v", err)
		}
		values.WaitPoint = val
	}

	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, values)
	if err != nil {
		return "", fmt.Errorf("error generating primary.cnf: %v", err)
	}

	return buf.String(), nil
}

type PrimarySqlUser struct {
	Username string
	Password string
}

type PrimarySqlOpts struct {
	ReplUser     string
	ReplPassword string

	Users     []PrimarySqlUser
	Databases []string
}

func PrimarySql(opts PrimarySqlOpts) (string, error) {
	tpl := createTpl("primary.sql", `CREATE USER '{{ .ReplUser }}'@'%' IDENTIFIED BY '{{ .ReplPassword }}';
GRANT REPLICATION REPLICA ON *.* TO 'repl'@'%';
{{ range $i, $user := .Users -}}
CREATE USER '{{ $user.Username }}'@'%' IDENTIFIED BY '{{ $user.Password }}';
GRANT ALL PRIVILEGES ON *.* TO '{{ $user.Username }}'@'%';
{{ end -}}
{{ range $i, $database := .Databases -}}
CREATE DATABASE {{ $database }};
{{ end -}}
`)

	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, opts)
	if err != nil {
		return "", fmt.Errorf("error generating primary.sql: %v", err)
	}

	return buf.String(), nil
}

func ReplicaCnf(replication mariadbv1alpha1.Replication) (string, error) {
	type tplValues struct {
		SemiSync bool
	}
	tpl := createTpl("replica.cnf", `[mariadb]
read_only=1
log-basename=mariadb
{{ if .SemiSync -}}
rpl_semi_sync_slave_enabled=ON
{{ end -}}
`)

	values := tplValues{
		SemiSync: replication.Mode == mariadbv1alpha1.ReplicationModeSemiSync,
	}

	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, values)
	if err != nil {
		return "", fmt.Errorf("error generating primary.cnf: %v", err)
	}

	return buf.String(), nil
}

type ReplicaSqlOpts struct {
	Meta     metav1.ObjectMeta
	User     string
	Password string
	Retries  *int32
}

func ReplicaSql(opts ReplicaSqlOpts) (string, error) {
	type tplValues struct {
		MasterHost string
		User       string
		Password   string
		Retries    int32
	}
	tpl := createTpl("replica.sql", `CHANGE MASTER TO 
MASTER_HOST='{{ .MasterHost }}',
MASTER_USER='{{ .User }}',
MASTER_PASSWORD='{{ .Password }}',
MASTER_CONNECT_RETRY={{ or .Retries 10 }};
`)

	values := tplValues{
		MasterHost: statefulset.PodFQDN(opts.Meta, 0),
		User:       opts.User,
		Password:   opts.Password,
	}
	if opts.Retries != nil {
		values.Retries = *opts.Retries
	}

	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, values)
	if err != nil {
		return "", fmt.Errorf("error generating replica.sql: %v", err)
	}

	return buf.String(), nil
}

type InitShOpts struct {
	PrimaryCnf string
	PrimarySql string
	ReplicaCnf string
	ReplicaSql string
}

func InitSh(opts InitShOpts) (string, error) {
	tpl := createTpl("init.sh", `#!/bin/bash

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
	cp /mnt/repl/{{ .PrimaryCnf }} /etc/mysql/conf.d/server-id.cnf
	cp /mnt/repl/{{ .PrimarySql }} /docker-entrypoint-initdb.d
else
	echo '🪞 Syncing replica config files'
	cp /mnt/repl/{{ .ReplicaCnf }} /etc/mysql/conf.d/server-id.cnf
	cp /mnt/repl/{{ .ReplicaSql }} /docker-entrypoint-initdb.d
fi

echo server-id=$((1000 + $ordinal)) >> /etc/mysql/conf.d/server-id.cnf
echo '🔧 /etc/mysql/conf.d/'
ls /etc/mysql/conf.d/
echo '🔧 /etc/mysql/conf.d/server-id.cnf'
cat /etc/mysql/conf.d/server-id.cnf
`)

	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, opts)
	if err != nil {
		return "", fmt.Errorf("error generating init.sh: %v", err)
	}

	return buf.String(), nil
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
