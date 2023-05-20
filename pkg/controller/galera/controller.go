package galera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/controller/configmap"
	galeraresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/galera/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GaleraReconciler struct {
	client.Client
	ConfigMapReconciler *configmap.ConfigMapReconciler
}

func NewGaleraReconciler(client client.Client, configMapReconciler *configmap.ConfigMapReconciler) *GaleraReconciler {
	return &GaleraReconciler{
		Client:              client,
		ConfigMapReconciler: configMapReconciler,
	}
}

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	if mariadb.Spec.Galera == nil || mariadb.IsRestoringBackup() {
		return nil
	}
	healthy, err := health.IsMariaDBHealthy(ctx, r.Client, mariadb, health.EndpointPolicyAll)
	if err != nil {
		return fmt.Errorf("error checking MariaDB health: %v", err)
	}
	if !healthy {
		return nil
	}
	return nil
}

func (r *GaleraReconciler) ReconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	galeraCnf, err := galeraConfig(mariadb)
	if err != nil {
		return fmt.Errorf("error generating galera config file: %v", err)
	}

	req := configmap.ReconcileRequest{
		Mariadb: mariadb,
		Owner:   mariadb,
		Key:     galeraresources.ConfigMapKey(mariadb),
		Data: map[string]string{
			"0-galera.cnf": galeraCnf,
		},
	}
	if err := r.ConfigMapReconciler.Reconcile(ctx, &req); err != nil {
		return fmt.Errorf("error reconciling ConfigMap: %v", err)
	}
	return nil
}

func galeraConfig(mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	tpl := createTpl("galera", `# Cluster configuration - rendered by mariadb-operator
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

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
