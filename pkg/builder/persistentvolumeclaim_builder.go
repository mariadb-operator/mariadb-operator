package builder

import (
	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (b *Builder) BuildPVC(meta metav1.ObjectMeta, storage *databasev1alpha1.Storage) *v1.PersistentVolumeClaim {
	accessModes := storage.AccessModes
	if accessModes == nil {
		accessModes = []v1.PersistentVolumeAccessMode{
			v1.ReadWriteOnce,
		}
	}

	return &v1.PersistentVolumeClaim{
		ObjectMeta: meta,
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      accessModes,
			StorageClassName: &storage.ClassName,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: storage.Size,
				},
			},
		},
	}
}
