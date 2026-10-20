package v1alpha1

import (
	"testing"

	"github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestGaleraConfigMetadataUpdate(t *testing.T) {
	old := &v1alpha1.MariaDB{
		Spec: v1alpha1.MariaDBSpec{
			Replicas: 3,
			Storage:  v1alpha1.Storage{Size: ptr.To(resource.MustParse("1Gi"))},
			Galera:   &v1alpha1.Galera{Enabled: true},
		},
	}
	old.Spec.Galera.Config.SetDefaults()
	validator := &MariaDBCustomValidator{}
	_, err := validator.ValidateUpdate(t.Context(), old, old.DeepCopy())
	require.NoError(t, err)

	for _, tt := range []struct {
		name    string
		mutate  func(*v1alpha1.GaleraConfig)
		wantErr bool
	}{
		{"add metadata", func(config *v1alpha1.GaleraConfig) {
			config.VolumeClaimTemplate.Metadata = &v1alpha1.Metadata{
				Labels:      map[string]string{"velero.io/exclude-from-backup": "true"},
				Annotations: map[string]string{"example.com/backup": "disabled"},
			}
		}, false},
		{"resize", func(config *v1alpha1.GaleraConfig) {
			config.VolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		}, true},
		{"storage class", func(config *v1alpha1.GaleraConfig) {
			config.VolumeClaimTemplate.StorageClassName = ptr.To("other")
		}, true},
		{"access modes", func(config *v1alpha1.GaleraConfig) {
			config.VolumeClaimTemplate.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		}, true},
		{"remove template", func(config *v1alpha1.GaleraConfig) {
			config.VolumeClaimTemplate = nil
		}, true},
		{"reuse storage", func(config *v1alpha1.GaleraConfig) {
			config.ReuseStorageVolume = ptr.To(true)
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			updated := old.DeepCopy()
			tt.mutate(&updated.Spec.Galera.Config)
			_, err := validator.ValidateUpdate(t.Context(), old, updated)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			_, err = validator.ValidateUpdate(t.Context(), updated, old.DeepCopy())
			require.NoError(t, err, "metadata can also be removed")
			changed := updated.DeepCopy()
			changed.Spec.Galera.Config.VolumeClaimTemplate.Metadata.Labels["velero.io/exclude-from-backup"] = "false"
			_, err = validator.ValidateUpdate(t.Context(), updated, changed)
			require.NoError(t, err)
		})
	}

	uninitialized := old.DeepCopy()
	uninitialized.Spec.Galera.Config.VolumeClaimTemplate = nil
	_, err = validator.ValidateUpdate(t.Context(), uninitialized, old)
	require.NoError(t, err, "initial template defaulting is still allowed")
}
