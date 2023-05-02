package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ConfigMapOpts struct {
	MariaDB *mariadbv1alpha1.MariaDB
	Key     types.NamespacedName
	Data    map[string]string
}

func (b *Builder) BuildConfigMap(opts ConfigMapOpts, owner metav1.Object) (*corev1.ConfigMap, error) {
	objLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(opts.MariaDB).
			Build()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
			Labels:    objLabels,
		},
		Data: opts.Data,
	}
	if err := controllerutil.SetControllerReference(owner, cm, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ConfigMap: %v", err)
	}
	return cm, nil
}
