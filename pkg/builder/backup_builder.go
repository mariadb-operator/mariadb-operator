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

func (b *Builder) BuildRestore(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*mariadbv1alpha1.Restore, error) {
	objLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			WithOwner(mariadb).
			Build()
	restore := &mariadbv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    objLabels,
		},
		Spec: mariadbv1alpha1.RestoreSpec{
			RestoreSource: *mariadb.Spec.BootstrapFrom,
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mariadb.Name,
				},
				WaitForIt: true,
			},
			Affinity:     mariadb.Spec.Affinity,
			NodeSelector: mariadb.Spec.NodeSelector,
			Tolerations:  mariadb.Spec.Tolerations,
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, restore, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to bootstrapping restore Job: %v", err)
	}
	return restore, nil
}
