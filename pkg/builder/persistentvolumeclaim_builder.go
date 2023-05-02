package builder

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (b *Builder) BuildPVC(key types.NamespacedName, storage *mariadbv1alpha1.BackupStorage,
	mariadb *mariadbv1alpha1.MariaDB) *v1.PersistentVolumeClaim {
	objLabels :=
		labels.NewLabelsBuilder().
			WithMariaDB(mariadb).
			WithOwner(mariadb).
			Build()
	objMeta := metav1.ObjectMeta{
		Name:      key.Name,
		Namespace: key.Namespace,
		Labels:    objLabels,
	}
	return &v1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       *storage.PersistentVolumeClaim,
	}
}
