package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildMaxScale(key types.NamespacedName, mdb *mariadbv1alpha1.MariaDB,
	baseSpec *mariadbv1alpha1.MaxScaleBaseSpec) (*mariadbv1alpha1.MaxScale, error) {
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mdb).
			Build()
	mxs := mariadbv1alpha1.MaxScale{
		ObjectMeta: objMeta,
		Spec: mariadbv1alpha1.MaxScaleSpec{
			MariaDBRef: &mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name:      mdb.Name,
					Namespace: mdb.Namespace,
				},
			},
			MaxScaleBaseSpec: mdb.Spec.MaxScale.MaxScaleBaseSpec,
		},
	}
	if err := controllerutil.SetControllerReference(mdb, &mxs, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller to MaxScale %v", err)
	}
	return &mxs, nil
}
