package builders

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretOpts struct {
	Name string
	Data map[string][]byte
}

func BuildSecret(mariadb *databasev1alpha1.MariaDB, opts SecretOpts) *corev1.Secret {
	labels := NewLabelsBuilder().WithObjectMeta(mariadb.ObjectMeta).WithApp(app).Build()
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: mariadb.Namespace,
			Labels:    labels,
		},
		Data: opts.Data,
	}
}
