package secret

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	password, err := password.Generate(16, 4, 2, false, false)
	if err != nil {
		return "", fmt.Errorf("error generating replication password: %v", err)
	}

	opts := builder.SecretOpts{
		MariaDB: mariadb,
		Key:     key,
		Data: map[string][]byte{
			secretKey: []byte(password),
		},
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

type ReconcileRequest struct {
	Owner metav1.Object
	Key   types.NamespacedName
	Data  map[string][]byte
}

func (r *SecretReconciler) Reconcile(ctx context.Context, req *ReconcileRequest) error {
	var existingSecret corev1.Secret
	err := r.Get(ctx, req.Key, &existingSecret)
	if err == nil {
		return nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting ConfigMap: %v", err)
	}

	secretOpts := builder.SecretOpts{
		Key:  req.Key,
		Data: req.Data,
	}
	secret, err := r.Builder.BuildSecret(secretOpts, req.Owner)
	if err != nil {
		return fmt.Errorf("error building Secret: %v", err)
	}

	return r.Create(ctx, secret)
}
