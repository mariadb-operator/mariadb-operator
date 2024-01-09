package metadata

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type MetadataBuilder struct {
	objMeta metav1.ObjectMeta
}

func NewMetadataBuilder(key types.NamespacedName) *MetadataBuilder {
	return &MetadataBuilder{
		objMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
}

func (b *MetadataBuilder) WithMariaDB(mariadb *mariadbv1alpha1.MariaDB) *MetadataBuilder {
	if mariadb == nil {
		return b
	}
	if mariadb.Spec.InheritMetadata == nil {
		return b
	}
	for k, v := range mariadb.Spec.InheritMetadata.Labels {
		b.objMeta.Labels[k] = v
	}
	for k, v := range mariadb.Spec.InheritMetadata.Annotations {
		b.objMeta.Annotations[k] = v
	}
	return b
}

func (b *MetadataBuilder) WithLabels(labels map[string]string) *MetadataBuilder {
	for k, v := range labels {
		b.objMeta.Labels[k] = v
	}
	return b
}

func (b *MetadataBuilder) WithAnnotations(annotations map[string]string) *MetadataBuilder {
	for k, v := range annotations {
		b.objMeta.Annotations[k] = v
	}
	return b
}

func (b *MetadataBuilder) Build() metav1.ObjectMeta {
	return b.objMeta
}
