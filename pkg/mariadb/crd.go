package mariadb

import (
	"context"
	"fmt"
	"os"

	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
)

func NewRootClientWithCrd(ctx context.Context, crd *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver) (*Client, error) {
	password, err := refResolver.SecretKeyRef(ctx, crd.Spec.RootPasswordSecretKeyRef, crd.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	opts := Opts{
		Username: "root",
		Password: password,
		Host:     FQDN(crd),
		Port:     crd.Spec.Port,
	}
	return NewClient(opts)
}

func FQDN(crd *mariadbv1alpha1.MariaDB) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", crd.Name, crd.Namespace, clusterName)
}
