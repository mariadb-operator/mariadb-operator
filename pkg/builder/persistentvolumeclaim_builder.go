package builder

import (
	mariadbv1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (b *Builder) BuildPVC(meta metav1.ObjectMeta, storage *mariadbv1alpha1.BackupStorage) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: meta,
		Spec:       *storage.PersistentVolumeClaim,
	}
}
