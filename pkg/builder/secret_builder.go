package builder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SecretOpts struct {
	Key         types.NamespacedName
	Data        map[string][]byte
	Labels      map[string]string
	Annotations map[string]string
}

func (b *Builder) BuildSecret(opts SecretOpts, owner metav1.Object) (*corev1.Secret, error) {
	objMeta := metav1.ObjectMeta{
		Name:        opts.Key.Name,
		Namespace:   opts.Key.Namespace,
		Labels:      opts.Labels,
		Annotations: opts.Annotations,
	}
	secret := &corev1.Secret{
		ObjectMeta: objMeta,
		Data:       opts.Data,
	}
	if err := controllerutil.SetControllerReference(owner, secret, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Secret: %v", err)
	}
	return secret, nil
}
