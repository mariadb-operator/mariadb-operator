package builder

import (
	"errors"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (b *Builder) BuildBackupPVC(key types.NamespacedName, backup *mariadbv1alpha1.Backup) (*corev1.PersistentVolumeClaim, error) {
	if backup.Spec.Storage.PersistentVolumeClaim == nil {
		return nil, errors.New("Backup spec does not have a PVC spec")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(backup.Spec.InheritMetadata).
			Build()
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       *backup.Spec.Storage.PersistentVolumeClaim,
	}, nil
}

func (b *Builder) BuildStoragePVC(key types.NamespacedName, tpl *mariadbv1alpha1.VolumeClaimTemplate,
	mariadb *mariadbv1alpha1.MariaDB) (*corev1.PersistentVolumeClaim, error) {
	if tpl == nil {
		return nil, errors.New("Template must not be nil")
	}
	labels := labels.NewLabelsBuilder().
		WithMariaDBSelectorLabels(mariadb).
		WithPVCRole(StorageVolumeRole).
		Build()
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(mariadb.Spec.InheritMetadata).
			WithMetadata(tpl.Metadata).
			WithLabels(labels).
			Build()
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       tpl.PersistentVolumeClaimSpec,
	}, nil
}
