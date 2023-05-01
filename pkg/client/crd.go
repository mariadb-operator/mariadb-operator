package client

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	replresources "github.com/mariadb-operator/mariadb-operator/pkg/controller/replication/resources"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewRootClient(ctx context.Context, crd *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver) (*Client, error) {
	password, err := refResolver.SecretKeyRef(ctx, crd.Spec.RootPasswordSecretKeyRef, crd.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	opts := Opts{
		Username: "root",
		Password: password,
		Host: func() string {
			if crd.Spec.Replication != nil {
				key := replresources.PrimaryServiceKey(crd)
				objMeta := metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				}
				return statefulset.ServiceFQDN(objMeta)
			}
			return statefulset.ServiceFQDN(crd.ObjectMeta)
		}(),
		Port: crd.Spec.Port,
	}
	return NewClient(opts)
}

func NewRootClientWithPodIndex(ctx context.Context, crd *mariadbv1alpha1.MariaDB, refResolver *refresolver.RefResolver,
	podIndex int) (*Client, error) {
	password, err := refResolver.SecretKeyRef(ctx, crd.Spec.RootPasswordSecretKeyRef, crd.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	opts := Opts{
		Username: "root",
		Password: password,
		Host:     statefulset.PodFQDN(crd.ObjectMeta, podIndex),
		Port:     crd.Spec.Port,
	}
	return NewClient(opts)
}
