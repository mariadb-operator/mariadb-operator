package refresolver

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type RefResolver struct {
	client client.Client
}

func New(client client.Client) *RefResolver {
	return &RefResolver{
		client: client,
	}
}

// TODO: use generics when kubebuilder has support

func (r *RefResolver) GetMariaDB(ctx context.Context, localRef corev1.LocalObjectReference,
	namespace string) (*databasev1alpha1.MariaDB, error) {
	var mariadb databasev1alpha1.MariaDB
	nn := types.NamespacedName{
		Name:      localRef.Name,
		Namespace: namespace,
	}
	if err := r.client.Get(ctx, nn, &mariadb); err != nil {
		return nil, err
	}
	return &mariadb, nil
}

func (r *RefResolver) GetBackupMariaDB(ctx context.Context, localRef corev1.LocalObjectReference,
	namespace string) (*databasev1alpha1.BackupMariaDB, error) {
	var backup databasev1alpha1.BackupMariaDB
	nn := types.NamespacedName{
		Name:      localRef.Name,
		Namespace: namespace,
	}
	if err := r.client.Get(ctx, nn, &backup); err != nil {
		return nil, err
	}
	return &backup, nil
}
