package galera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
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

func (r *GaleraReconciler) ReconcileService(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	key := galeraresources.ServiceKey(mariadb)
	clusterIp := "None"
	publishNotReadyAddresses := true
	opts := builder.ServiceOpts{
		Type: corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{
			{
				Name: galeraresources.GaleraClusterPortName,
				Port: galeraresources.GaleraClusterPort,
			},
			{
				Name: galeraresources.GaleraISTPortName,
				Port: galeraresources.GaleraISTPort,
			},
			{
				Name: galeraresources.GaleraSSTPortName,
				Port: galeraresources.GaleraSSTPort,
			},
			{
				Name: galeraresources.AgentPortName,
				Port: mariadb.Spec.Galera.Agent.Port,
			},
		},
		ClusterIP:                &clusterIp,
		PublishNotReadyAddresses: &publishNotReadyAddresses,
	}
	if mariadb.Spec.Service != nil {
		opts.Annotations = mariadb.Spec.Service.Annotations
	}
	desiredSvc, err := r.Builder.BuildService(mariadb, key, opts)
	if err != nil {
		return fmt.Errorf("error building Service: %v", err)
	}
	return r.ServiceReconciler.Reconcile(ctx, desiredSvc)
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
wsrep_node_address="$MARIADB_OPERATOR_HOSTNAME.{{ .Service }}"
wsrep_node_name="$MARIADB_OPERATOR_HOSTNAME"
`)
	buf := new(bytes.Buffer)
	clusterAddr, err := clusterAddress(mariadb)
	if err != nil {
		return "", fmt.Errorf("error getting cluster address: %v", err)
	}
	err = tpl.Execute(buf, struct {
		ClusterAddress string
		Threads        int
		Service        string
	}{
		ClusterAddress: clusterAddr,
		Threads:        mariadb.Spec.Galera.ReplicaThreads,
		Service:        statefulset.ServiceFQDNWithService(mariadb.ObjectMeta, galeraresources.ServiceKey(mariadb).Name),
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
		pods[i] = statefulset.PodFQDNWithService(mariadb.ObjectMeta, i, galeraresources.ServiceKey(mariadb).Name)
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
cat {{ .ConfigMapPath }}/{{ .GaleraCnf }} | sed 's/$MARIADB_OPERATOR_HOSTNAME/'$HOSTNAME'/g' > {{ .ConfigPath }}/{{ .GaleraCnf }}

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
			galeraresources.ServiceKey(mariadb).Name,
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
