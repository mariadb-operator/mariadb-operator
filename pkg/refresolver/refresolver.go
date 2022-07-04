package refresolver

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

type RefResolver struct {
	client client.Client
}

func New(client client.Client) *RefResolver {
	return &RefResolver{
		client: client,
	}
}

func (r *RefResolver) GetMariaDB(ctx context.Context, localRef corev1.LocalObjectReference,
	namespace string) (*databasev1alpha1.MariaDB, error) {
	nn := types.NamespacedName{
		Name:      localRef.Name,
		Namespace: namespace,
	}
	var mariadb databasev1alpha1.MariaDB
	if err := r.client.Get(ctx, nn, &mariadb); err != nil {
		return nil, err
	}
	return &mariadb, nil
}

func (r *RefResolver) GetBackupMariaDB(ctx context.Context, localRef corev1.LocalObjectReference,
	namespace string) (*databasev1alpha1.BackupMariaDB, error) {
	nn := types.NamespacedName{
		Name:      localRef.Name,
		Namespace: namespace,
	}
	var backup databasev1alpha1.BackupMariaDB
	if err := r.client.Get(ctx, nn, &backup); err != nil {
		return nil, err
	}
	return &backup, nil
}

func (r *RefResolver) ReadSecretKeyRef(ctx context.Context, selector corev1.SecretKeySelector,
	namespace string) (string, error) {
	nn := types.NamespacedName{
		Name:      selector.Name,
		Namespace: namespace,
	}
	var secret v1.Secret
	if err := r.client.Get(ctx, nn, &secret); err != nil {
		return "", fmt.Errorf("error getting secret: %v", err)
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("secret key \"%s\" not found", selector.Key)
	}

	return string(data), nil
}
