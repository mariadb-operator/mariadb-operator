package secret

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretReconciler struct {
	client.Client
	Builder *builder.Builder
}

func NewSecretReconciler(client client.Client, builder *builder.Builder) *SecretReconciler {
	return &SecretReconciler{
		Client:  client,
		Builder: builder,
	}
}

func (r *SecretReconciler) ReconcileRandomPassword(ctx context.Context, key types.NamespacedName, secretKey string,
	mariadb *mariadbv1alpha1.MariaDB) (string, error) {
	var existingSecret corev1.Secret
	if err := r.Get(ctx, key, &existingSecret); err == nil {
		return string(existingSecret.Data[secretKey]), nil
	}
	password, err := password.Generate(16, 4, 0, false, false)
	if err != nil {
		return "", fmt.Errorf("error generating replication password: %v", err)
	}

	opts := builder.SecretOpts{
		Key: key,
		Data: map[string][]byte{
			secretKey: []byte(password),
		},
		Labels: labels.NewLabelsBuilder().WithMariaDB(mariadb).Build(),
	}
	secret, err := r.Builder.BuildSecret(opts, mariadb)
	if err != nil {
		return "", fmt.Errorf("error building replication password Secret: %v", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return "", fmt.Errorf("error creating replication password Secret: %v", err)
	}

	return password, nil
}
