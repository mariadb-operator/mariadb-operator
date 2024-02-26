package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildRestore(mariadb *mariadbv1alpha1.MariaDB, key types.NamespacedName) (*mariadbv1alpha1.Restore, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	restore := &mariadbv1alpha1.Restore{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.RestoreSpec{
			ContainerTemplate: mariadb.Spec.ContainerTemplate,
			PodTemplate:       mariadb.Spec.PodTemplate,
			RestoreSource:     *mariadb.Spec.BootstrapFrom,
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name: mariadb.Name,
				},
				WaitForIt: true,
			},
		},
	}
	if err := controllerutil.SetControllerReference(mariadb, restore, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to restore Job: %v", err)
	}
	return restore, nil
}
