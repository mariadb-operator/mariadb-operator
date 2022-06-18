package refreader

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RefReader struct {
	client client.Client
}

func New(client client.Client) *RefReader {
	return &RefReader{
		client: client,
	}
}

func (r *RefReader) ReadSecretKeyRef(ctx context.Context, selector corev1.SecretKeySelector, namespace string) (string, error) {
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
