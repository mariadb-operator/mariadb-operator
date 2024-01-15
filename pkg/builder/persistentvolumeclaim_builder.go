package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (b *Builder) BuildBackupPVC(key types.NamespacedName, storage *mariadbv1alpha1.BackupStorage,
	mariadb *mariadbv1alpha1.MariaDB) (*v1.PersistentVolumeClaim, error) {
	if storage.PersistentVolumeClaim == nil {
		return nil, fmt.Errorf("Backup spec does not have a PVC spec")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMariaDB(mariadb).
			Build()
	return &v1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       *storage.PersistentVolumeClaim,
	}, nil
}
