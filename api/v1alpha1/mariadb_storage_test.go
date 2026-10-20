package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestStorageDefaultsPreserveMetadata(t *testing.T) {
	meta := &Metadata{
		Labels:      map[string]string{"velero.io/exclude-from-backup": "true"},
		Annotations: map[string]string{"ebs.csi.aws.com/volumeType": "gp3"},
	}
	storage := Storage{
		Size:                ptr.To(resource.MustParse("1Gi")),
		StorageClassName:    "standard",
		VolumeClaimTemplate: &VolumeClaimTemplate{Metadata: meta.DeepCopy()},
	}
	storage.SetDefaults()
	require.Equal(t, meta, storage.VolumeClaimTemplate.Metadata)
	require.Equal(t, "standard", *storage.VolumeClaimTemplate.StorageClassName)
	require.Equal(t, resource.MustParse("1Gi"), storage.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage])

	storage.Size = ptr.To(resource.MustParse("2Gi"))
	storage.SetDefaults()
	require.Equal(t, meta, storage.VolumeClaimTemplate.Metadata)
	require.Equal(t, resource.MustParse("2Gi"), storage.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage])

	storage.StorageClassName = "other"
	storage.SetDefaults()
	require.Equal(t, meta, storage.VolumeClaimTemplate.Metadata)
	require.Equal(t, "other", *storage.VolumeClaimTemplate.StorageClassName)
}
