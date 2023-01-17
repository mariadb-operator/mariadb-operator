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

func (b *Builder) BuildRestore(mariaDb *mariadbv1alpha1.MariaDB, restoreSource *mariadbv1alpha1.RestoreSource,
	key types.NamespacedName) (*mariadbv1alpha1.Restore, error) {
	restoreLabels :=
		labels.NewLabelsBuilder().
			WithApp(appMariaDb).
			WithInstance(mariaDb.Name).
			Build()
	restore := &mariadbv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    restoreLabels,
		},
		Spec: mariadbv1alpha1.RestoreSpec{
			RestoreSource: *restoreSource,
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mariaDb.Name,
				},
				WaitForIt: true,
			},
		},
	}
	if err := controllerutil.SetControllerReference(mariaDb, restore, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to bootstrapping restore Job: %v", err)
	}
	return restore, nil
}
