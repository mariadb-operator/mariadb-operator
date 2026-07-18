package metadata

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	pkgmetadata "github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"k8s.io/apimachinery/pkg/types"
)

func TestMetadataBuilderSetsManagedByLabel(t *testing.T) {
	meta := NewMetadataBuilder(types.NamespacedName{
		Name:      "test",
		Namespace: "default",
	}).Build()

	if got := meta.Labels[pkgmetadata.KubernetesManagedByLabel]; got != pkgmetadata.KubernetesManagedByValue {
		t.Fatalf("expected %s=%s, got %q", pkgmetadata.KubernetesManagedByLabel, pkgmetadata.KubernetesManagedByValue, got)
	}
}

func TestMetadataBuilderPreservesManagedByLabel(t *testing.T) {
	meta := NewMetadataBuilder(types.NamespacedName{
		Name:      "test",
		Namespace: "default",
	}).
		WithMetadata(&mariadbv1alpha1.Metadata{
			Labels: map[string]string{
				pkgmetadata.KubernetesManagedByLabel: "custom-value",
			},
		}).
		Build()

	if got := meta.Labels[pkgmetadata.KubernetesManagedByLabel]; got != pkgmetadata.KubernetesManagedByValue {
		t.Fatalf("expected %s=%s, got %q", pkgmetadata.KubernetesManagedByLabel, pkgmetadata.KubernetesManagedByValue, got)
	}
}
