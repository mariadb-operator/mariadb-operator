package builder

import (
	"errors"
	"fmt"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	metadata "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/metadata"
	mdbreflect "github.com/mariadb-operator/mariadb-operator/v25/pkg/reflect"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (b *Builder) BuildBackupStoragePVC(key types.NamespacedName, pvcSpec *mariadbv1alpha1.PersistentVolumeClaimSpec,
	meta *mariadbv1alpha1.Metadata) (*corev1.PersistentVolumeClaim, error) {
	if pvcSpec == nil {
		return nil, errors.New("PVC spec must be set")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(meta).
			Build()
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       pvcSpec.ToKubernetesType(),
	}, nil
}

func (b *Builder) BuildBackupStagingPVC(key types.NamespacedName, pvcSpec *mariadbv1alpha1.PersistentVolumeClaimSpec,
	meta *mariadbv1alpha1.Metadata, owner metav1.Object) (*corev1.PersistentVolumeClaim, error) {
	if pvcSpec == nil {
		return nil, errors.New("PVC spec must be set")
	}
	objMeta :=
		metadata.NewMetadataBuilder(key).
			WithMetadata(meta).
			Build()
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       pvcSpec.ToKubernetesType(),
	}
	if !mdbreflect.IsNil(owner) {
		if err := controllerutil.SetControllerReference(owner, &pvc, b.scheme); err != nil {
			return nil, fmt.Errorf("error setting controller to PVC %v", err)
		}
	}
	return &pvc, nil
}

type PVCOption func(*corev1.PersistentVolumeClaimSpec)

func WithVolumeSnapshotDataSource(snapshotName string) PVCOption {
	return func(pvcSpec *corev1.PersistentVolumeClaimSpec) {
		pvcSpec.DataSource = &corev1.TypedLocalObjectReference{
			APIGroup: ptr.To(volumesnapshotv1.GroupName),
			Kind:     "VolumeSnapshot",
			Name:     snapshotName,
		}
	}
}

func (b *Builder) BuildStoragePVC(key types.NamespacedName, tpl *mariadbv1alpha1.VolumeClaimTemplate,
	mariadb *mariadbv1alpha1.MariaDB, opts ...PVCOption) (*corev1.PersistentVolumeClaim, error) {
	if tpl == nil {
		return nil, errors.New("template must not be nil")
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

	pvcSpec := tpl.ToKubernetesType()
	for _, setOpt := range opts {
		setOpt(&pvcSpec)
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec:       pvcSpec,
	}, nil
}
