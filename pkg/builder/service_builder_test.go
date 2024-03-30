package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func TestServiceMeta(t *testing.T) {
	builder := newTestBuilder()
	key := types.NamespacedName{
		Name: "service",
	}
	tests := []struct {
		name     string
		opts     ServiceOpts
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			opts: ServiceOpts{
				Metadata:              &mariadbv1alpha1.Metadata{},
				ExcludeSelectorLabels: true,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
			opts: ServiceOpts{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
				ExcludeSelectorLabels: true,
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
			configMap, err := builder.BuildService(key, &mariadbv1alpha1.MariaDB{}, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error building Service: %v", err)
			}
			assertMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
