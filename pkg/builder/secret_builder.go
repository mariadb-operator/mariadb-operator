package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	labels "github.com/mmontes11/mariadb-operator/pkg/builder/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SecretOpts struct {
	Key  types.NamespacedName
	Data map[string][]byte
}

func (b *Builder) BuildSecret(mariadb *mariadbv1alpha1.MariaDB, opts SecretOpts) (*corev1.Secret, error) {
	secretLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
			Labels:    secretLabels,
		},
		Data: opts.Data,
	}
	if err := controllerutil.SetControllerReference(mariadb, secret, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to password Secret: %v", err)
	}
	return secret, nil
}
