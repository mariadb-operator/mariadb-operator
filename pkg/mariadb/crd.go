package mariadb

import (
	"context"
	"fmt"
	"os"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
)

func NewRootClientWithCrd(ctx context.Context, crd *databasev1alpha1.MariaDB, refResolver *refresolver.RefResolver) (*Client, error) {
	password, err := refResolver.ReadSecretKeyRef(ctx, crd.Spec.RootPasswordSecretKeyRef, crd.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	opts := Opts{
		Username: "root",
		Password: password,
		Host:     GetDNS(crd),
		Port:     crd.Spec.Port,
	}
	return NewClient(opts)
}

func GetDNS(crd *databasev1alpha1.MariaDB) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", crd.Name, crd.Namespace, clusterName)
}
