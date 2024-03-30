package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildEndpoints(key types.NamespacedName, mariadb *mariadbv1alpha1.MariaDB,
	subsets []corev1.EndpointSubset) (*corev1.Endpoints, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			Build()
	endpoints := &corev1.Endpoints{
		ObjectMeta: objMeta,
		Subsets:    subsets,
	}
	if err := controllerutil.SetControllerReference(mariadb, endpoints, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Endpoints: %v", err)
	}
	return endpoints, nil
}
