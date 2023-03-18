package refresolver

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RefResolver struct {
	client client.Client
}

func New(client client.Client) *RefResolver {
	return &RefResolver{
		client: client,
	}
}

func (r *RefResolver) MariaDB(ctx context.Context, ref *mariadbv1alpha1.MariaDBRef,
	namespace string) (*mariadbv1alpha1.MariaDB, error) {
	nn := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	var mariadb mariadbv1alpha1.MariaDB
	if err := r.client.Get(ctx, nn, &mariadb); err != nil {
		return nil, err
	}
	return &mariadb, nil
}

func (r *RefResolver) Backup(ctx context.Context, ref *corev1.LocalObjectReference,
	namespace string) (*mariadbv1alpha1.Backup, error) {
	nn := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	var backup mariadbv1alpha1.Backup
	if err := r.client.Get(ctx, nn, &backup); err != nil {
		return nil, err
	}
	return &backup, nil
}

func (r *RefResolver) SqlJob(ctx context.Context, ref *corev1.LocalObjectReference,
	namespace string) (*mariadbv1alpha1.SqlJob, error) {
	nn := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	var sqlJob mariadbv1alpha1.SqlJob
	if err := r.client.Get(ctx, nn, &sqlJob); err != nil {
		return nil, err
	}
	return &sqlJob, nil
}

func (r *RefResolver) SecretKeyRef(ctx context.Context, selector corev1.SecretKeySelector,
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
