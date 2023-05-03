package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	appLabel           = "app.kubernetes.io/name"
	instanceLabel      = "app.kubernetes.io/instance"
	statefulSetPodName = "statefulset.kubernetes.io/pod-name"
	releaseLabel       = "release"
	appMariaDb         = "mariadb"
)

type LabelsBuilder struct {
	labels map[string]string
}

func NewLabelsBuilder() *LabelsBuilder {
	return &LabelsBuilder{
		labels: map[string]string{},
	}
}

func (b *LabelsBuilder) WithApp(app string) *LabelsBuilder {
	b.labels[appLabel] = app
	return b
}

func (b *LabelsBuilder) WithInstance(instance string) *LabelsBuilder {
	b.labels[instanceLabel] = instance
	return b
}

func (b *LabelsBuilder) WithRelease(release string) *LabelsBuilder {
	b.labels[releaseLabel] = release
	return b
}

func (b *LabelsBuilder) WithMariaDB(mdb *mariadbv1alpha1.MariaDB) *LabelsBuilder {
	return b.WithApp(appMariaDb).
		WithInstance(mdb.Name)
}

func (b *LabelsBuilder) WithOwner(owner metav1.Object) *LabelsBuilder {
	return b.WithLabels(owner.GetLabels())
}

func (b *LabelsBuilder) WithStatefulSetPod(mdb *mariadbv1alpha1.MariaDB, podIndex int) *LabelsBuilder {
	b.labels[statefulSetPodName] = statefulset.PodName(mdb.ObjectMeta, podIndex)
	return b
}

func (b *LabelsBuilder) WithLabels(labels map[string]string) *LabelsBuilder {
	for k, v := range labels {
		b.labels[k] = v
	}
	return b
}

func (b *LabelsBuilder) Build() map[string]string {
	return b.labels
}
