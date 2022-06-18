package builders

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	appLabel      = "app.kubernetes.io/name"
	instanceLabel = "app.kubernetes.io/instance"
)

type LabelsBuilder struct {
	labels map[string]string
}

func NewLabelsBuilder() *LabelsBuilder {
	return &LabelsBuilder{
		labels: map[string]string{},
	}
}

func (b *LabelsBuilder) WithObjectMeta(meta metav1.ObjectMeta) *LabelsBuilder {
	b.labels[instanceLabel] = meta.Name
	for k, v := range meta.Labels {
		b.labels[k] = v
	}
	return b
}

func (b *LabelsBuilder) WithApp(app string) *LabelsBuilder {
	b.labels[appLabel] = app
	return b
}

func (b *LabelsBuilder) Build() map[string]string {
	return b.labels
}
