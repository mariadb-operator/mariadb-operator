package builder

import (
	"fmt"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/metadata"
	"k8s.io/apimachinery/pkg/types"
)

func (b *Builder) BuildVolumeSnapshot(key types.NamespacedName, backup *mariadbv1alpha1.PhysicalBackup,
	pvcKey types.NamespacedName, meta *mariadbv1alpha1.Metadata) (*volumesnapshotv1.VolumeSnapshot, error) {
	if backup.Spec.Storage.VolumeSnapshot == nil {
		return nil, fmt.Errorf("VolumeSnapshot must be set as storage")
	}
	snapshotSpec := backup.Spec.Storage.VolumeSnapshot
	snapshotMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(backup.Spec.InheritMetadata).
			WithMetadata(snapshotSpec.Metadata).
			WithLabels(
				labels.NewLabelsBuilder().
					WithPhysicalBackupSelectorLabels(backup).
					Build(),
			).
			WithMetadata(meta).
			Build()

	return &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: snapshotMeta,
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			Source: volumesnapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvcKey.Name,
			},
			VolumeSnapshotClassName: &snapshotSpec.VolumeSnapshotClassName,
		},
	}, nil
}
