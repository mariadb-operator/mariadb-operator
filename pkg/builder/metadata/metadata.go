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

func (b *MetadataBuilder) WithReleaseLabel(release string) *MetadataBuilder {
	if release == "" {
		return b
	}
	return b.WithLabels(map[string]string{
		"release": release,
	})
}

func (b *MetadataBuilder) WithMetadata(meta *mariadbv1alpha1.Metadata) *MetadataBuilder {
	if meta == nil {
		return b
	}
	for k, v := range meta.Labels {
		b.objMeta.Labels[k] = v
	}
	for k, v := range meta.Annotations {
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
