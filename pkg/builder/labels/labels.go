package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	appLabel           = "app.kubernetes.io/name"
	instanceLabel      = "app.kubernetes.io/instance"
	statefulSetPodName = "statefulset.kubernetes.io/pod-name"
	volumeRole         = "pvc.k8s.mariadb.com/role"
	podRole            = "k8s.mariadb.com/role"
	appMariaDb         = "mariadb"
	appExporter        = "exporter"
	appMaxScale        = "maxscale"
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

func (b *LabelsBuilder) WithStatefulSetPod(objMeta metav1.ObjectMeta, podIndex int) *LabelsBuilder {
	b.labels[statefulSetPodName] = statefulset.PodName(objMeta, podIndex)
	return b
}

func (b *LabelsBuilder) WithLabels(labels map[string]string) *LabelsBuilder {
	for k, v := range labels {
		b.labels[k] = v
	}
	return b
}

func (b *LabelsBuilder) WithMariaDBSelectorLabels(mdb *mariadbv1alpha1.MariaDB) *LabelsBuilder {
	return b.WithApp(appMariaDb).
		WithInstance(mdb.Name)
}

func (b *LabelsBuilder) WithMetricsSelectorLabels(metricsKey types.NamespacedName) *LabelsBuilder {
	return b.WithApp(appExporter).
		WithInstance(metricsKey.Name)
}

func (b *LabelsBuilder) WithMaxScaleSelectorLabels(mxs *mariadbv1alpha1.MaxScale) *LabelsBuilder {
	return b.WithApp(appMaxScale).
		WithInstance(mxs.Name)
}

func (b *LabelsBuilder) WithPhysicalBackupSelectorLabels(backup *mariadbv1alpha1.PhysicalBackup) *LabelsBuilder {
	b.labels[metadata.PhysicalBackupNameLabel] = backup.Name
	return b
}

func (b *LabelsBuilder) WithPVCRole(role string) *LabelsBuilder {
	b.labels[volumeRole] = role
	return b
}

func (b *LabelsBuilder) WithPodRole(role string) *LabelsBuilder {
	b.labels[podRole] = role
	return b
}

func (b *LabelsBuilder) Build() map[string]string {
	return b.labels
}
