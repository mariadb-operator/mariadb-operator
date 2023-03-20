package mariadb

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
)

func NewRootClientWithCrd(ctx context.Context, crd *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver) (*Client, error) {
	password, err := refResolver.SecretKeyRef(ctx, crd.Spec.RootPasswordSecretKeyRef, crd.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	opts := Opts{
		Username: "root",
		Password: password,
		Host:     statefulset.PodFQDN(crd.ObjectMeta, 0),
		Port:     crd.Spec.Port,
	}
	return NewClient(opts)
}
