package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type SecretOpts struct {
	Key  types.NamespacedName
	Data map[string][]byte
}

func BuildSecret(mariadb *databasev1alpha1.MariaDB, opts SecretOpts) *corev1.Secret {
	labels :=
		NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariadb.Name).
			Build()
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
			Labels:    labels,
		},
		Data: opts.Data,
	}
}
