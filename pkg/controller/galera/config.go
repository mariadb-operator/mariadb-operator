package galera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ctrlresources "github.com/mariadb-operator/mariadb-operator/controllers/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
)

func (r *GaleraReconciler) ReconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	galeraCnf, err := galeraConfig(mariadb)
	if err != nil {
		return fmt.Errorf("error generating Galera config file: %v", err)
	}
	initSh, err := galeraInit(mariadb)
	if err != nil {
		return fmt.Errorf("error generating Galera init script: %v", err)
	}

	req := configmap.ReconcileRequest{
		Mariadb: mariadb,
		Owner:   mariadb,
		Key:     galeraresources.ConfigMapKey(mariadb),
		Data: map[string]string{
			galeraresources.GaleraCnf: galeraCnf,
			galeraresources.GaleraBootstrapCnf: `[mysqld]
wsrep_new_cluster="ON"
`,
			galeraresources.GaleraInitScript: initSh,
		},
	}
	return r.ConfigMapReconciler.Reconcile(ctx, &req)
}

func galeraConfig(mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	tpl := createTpl("galera", `[mysqld]
bind-address=0.0.0.0
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2

# Cluster configuration - rendered by mariadb-operator
wsrep_on=ON
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_cluster_address='{{ .ClusterAddress }}'
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads={{ .Threads }}

# Node configuration - to be rendered by initContainer
wsrep_node_address="$POD_HOSTNAME.{{ .Service }}"
wsrep_node_name="$POD_HOSTNAME"

# SST: https://mariadb.com/kb/en/introduction-to-state-snapshot-transfers-ssts/ - to be rendered by initContainer
wsrep_sst_method="{{ .SST }}"
{{- if .SSTAuth }}
wsrep_sst_auth="root:$MARIADB_ROOT_PASSWORD"
{{- end }}
`)
	buf := new(bytes.Buffer)
	clusterAddr, err := clusterAddress(mariadb)
	if err != nil {
		return "", fmt.Errorf("error getting cluster address: %v", err)
	}
	sst, err := mariadb.Spec.Galera.SST.MariaDBFormat()
	if err != nil {
		return "", fmt.Errorf("error getting SST: %v", err)
	}
	err = tpl.Execute(buf, struct {
		ClusterAddress string
		Threads        int
		Service        string
		SST            string
		SSTAuth        bool
	}{
		ClusterAddress: clusterAddr,
		Threads:        mariadb.Spec.Galera.ReplicaThreads,
		Service: statefulset.ServiceFQDNWithService(
			mariadb.ObjectMeta,
			ctrlresources.InternalServiceKey(mariadb).Name,
		),
		SST:     sst,
		SSTAuth: mariadb.Spec.Galera.SST == mariadbv1alpha1.SSTMariaBackup || mariadb.Spec.Galera.SST == mariadbv1alpha1.SSTMysqldump,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func clusterAddress(mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	if mariadb.Spec.Replicas == 0 {
		return "", errors.New("at least one replica must be specified to get a valid cluster address")
	}
	pods := make([]string, mariadb.Spec.Replicas)
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		pods[i] = statefulset.PodFQDNWithService(
			mariadb.ObjectMeta,
			i,
			ctrlresources.InternalServiceKey(mariadb).Name,
		)
	}
	return fmt.Sprintf("gcomm://%s", strings.Join(pods, ",")), nil
}

// TODO: refactor this into a Go container.
// This way we can query the Pod readiness using the Kubernetes API instead of replicating the probe
func galeraInit(mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	tpl := createTpl("init", `#!/bin/bash
set -euo pipefail
HOSTNAME=$(hostname)
STATEFULSET_INDEX=${HOSTNAME##*-}

echo 'Adding galera configuration'
cat {{ .ConfigMapPath }}/{{ .GaleraCnf }} | \
	sed 's/$POD_HOSTNAME/'$HOSTNAME'/g' | \
	sed 's/$MARIADB_ROOT_PASSWORD/'$MARIADB_ROOT_PASSWORD'/g' > {{ .ConfigPath }}/{{ .GaleraCnf }}

if [ ! -z "$(ls -A {{ .StoragePath }})" ]; then
	exit 0;
fi

if [ "$STATEFULSET_INDEX" = "0" ]; then 
	echo 'Adding cluster bootstrapping configuration'
	cp {{ .ConfigMapPath }}/{{ .GaleraBootstrapCnf }} {{ .ConfigPath }}/{{ .GaleraBootstrapCnf }}
else
	while true; do
		PREVIOUS_INDEX=$((STATEFULSET_INDEX - 1))
		HOST="{{ .MariaDBName }}-$PREVIOUS_INDEX.{{ .InternalServiceFQDN }}"
		COUNT=$(mysql -u root -p"$MARIADB_ROOT_PASSWORD" -e "SHOW STATUS LIKE 'wsrep_ready'" -h $HOST | grep -c ON || true)
		if [[ $COUNT -eq 1 ]]; then
			echo "$HOST is Ready. Exiting...";
			sleep 10s;
			exit 0;
		else
			echo "Waiting for $HOST to be Ready";
			sleep 1s;
		fi
	done;
fi
`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, struct {
		GaleraCnf           string
		GaleraBootstrapCnf  string
		ConfigMapPath       string
		ConfigPath          string
		StoragePath         string
		MariaDBName         string
		InternalServiceFQDN string
	}{
		GaleraCnf:          galeraresources.GaleraCnf,
		GaleraBootstrapCnf: galeraresources.GaleraBootstrapCnf,
		ConfigMapPath:      galeraresources.GaleraConfigMapMountPath,
		ConfigPath:         galeraresources.GaleraConfigMountPath,
		StoragePath:        builder.StorageMountPath,
		MariaDBName:        mariadb.Name,
		InternalServiceFQDN: statefulset.ServiceFQDNWithService(
			mariadb.ObjectMeta,
			ctrlresources.InternalServiceKey(mariadb).Name,
		),
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
