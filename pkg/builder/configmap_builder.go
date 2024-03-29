package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ConfigMapOpts struct {
	Metadata *mariadbv1alpha1.Metadata
	Key      types.NamespacedName
	Data     map[string]string
}

func (b *Builder) BuildConfigMap(opts ConfigMapOpts, owner metav1.Object) (*corev1.ConfigMap, error) {
	objMeta :=
		metadata.NewMetadataBuilder(opts.Key).
			WithMetadata(opts.Metadata).
			Build()
	cm := &corev1.ConfigMap{
		ObjectMeta: objMeta,
		Data:       opts.Data,
	}
	if err := controllerutil.SetControllerReference(owner, cm, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ConfigMap: %v", err)
	}
	return cm, nil
}
