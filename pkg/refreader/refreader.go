package refreader

import (
	"context"
	b64 "encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RefReader struct {
	client    client.Client
	namespace string
}

func (r *RefReader) ReadSecretKeyRef(ctx context.Context, selector corev1.SecretKeySelector) (string, error) {
	nn := types.NamespacedName{
		Name:      selector.Name,
		Namespace: r.namespace,
	}

	var secret v1.Secret
	if err := r.client.Get(ctx, nn, &secret); err != nil {
		return "", fmt.Errorf("error getting secret: %v", err)
	}

	encoded, ok := secret.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("secret key \"%s\" not found", selector.Key)
	}

	decoded, err := b64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", fmt.Errorf("error decoding secret: %v", err)
	}

	return string(decoded), nil
}

func New(client client.Client, namespace string) *RefReader {
	return &RefReader{
		client:    client,
		namespace: namespace,
	}
}
