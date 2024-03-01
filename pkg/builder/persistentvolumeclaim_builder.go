package builder

import (
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (b *Builder) BuildBackupPVC(key types.NamespacedName, storage *mariadbv1alpha1.BackupStorage,
	mariadb *mariadbv1alpha1.MariaDB) (*corev1.PersistentVolumeClaim, error) {
	if storage.PersistentVolumeClaim == nil {
		return nil, fmt.Errorf("Backup spec does not have a PVC spec")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       *storage.PersistentVolumeClaim,
	}, nil
}

func (b *Builder) BuildStoragePVC(key types.NamespacedName, tpl *mariadbv1alpha1.VolumeClaimTemplate,
	mariadb *mariadbv1alpha1.MariaDB) (*corev1.PersistentVolumeClaim, error) {
	if tpl == nil {
		return nil, errors.New("Template must not be nil")
	}
	labels := labels.NewLabelsBuilder().
		WithLabels(tpl.Labels).
		WithMariaDB(mariadb).
		WithPVCRole(StorageVolumeRole).
		Build()
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			WithLabels(labels).
			WithAnnotations(tpl.Annotations).
			Build()
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       tpl.PersistentVolumeClaimSpec,
	}, nil
}
