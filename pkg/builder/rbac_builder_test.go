package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func TestServiceAccountMeta(t *testing.T) {
	builder := newTestBuilder()
	key := types.NamespacedName{
		Name: "sa",
	}
	tests := []struct {
		name     string
		meta     *mariadbv1alpha1.Metadata
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			meta: nil,
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
			meta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa, err := builder.BuildServiceAccount(key, &mariadbv1alpha1.MariaDB{}, tt.meta)
			if err != nil {
				t.Fatalf("unexpected error building ServiceAccunt: %v", err)
			}
			assertMeta(t, &sa.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
