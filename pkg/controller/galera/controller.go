package galera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	galeraConfigMapKey = "0-galera.cnf"
)

type GaleraReconciler struct {
	client.Client
}

func NewGaleraReconciler(client client.Client) *GaleraReconciler {
	return &GaleraReconciler{
		Client: client,
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

	if err := r.reconcileConfigMap(ctx, mariadb); err != nil {
		return fmt.Errorf("error reconciling galera ConfigMap: %v", err)
	}
	return nil
}

func (r *GaleraReconciler) reconcileConfigMap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	_, err := galeraConfig(mariadb)
	if err != nil {
		return fmt.Errorf("error generating galera config file: %v", err)
	}
	return nil
}

func galeraConfig(mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	tpl := createTpl(galeraConfigMapKey, `# Cluster configuration - rendered by mariadb-operator
wsrep_on=ON
wsrep_provider=/usr/lib/galera/libgalera_smm.so
wsrep_cluster_address='{{ .ClusterAddress }}'
wsrep_cluster_name=mariadb-operator
wsrep_slave_threads={{ .Threads }}
# Node configuration - to be rendered by initContainer
wsrep_node_address="{{ HOSTNAME }}.{{ .Service }}"
wsrep_node_name="{{ HOSTNAME }}"
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
		Threads:        mariadb.Spec.Galera.Threads,
		Service:        statefulset.ServiceFQDN(mariadb.ObjectMeta),
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
	addr := "gcomm://"
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		addr += statefulset.PodFQDN(mariadb.ObjectMeta, i)
	}
	return addr, nil
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
