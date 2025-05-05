package builder

import (
	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
)

func TestPodDisruptionBudgetMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tests := []struct {
		name     string
		opts     PodDisruptionBudgetOpts
		wantMeta *v1alpha1.Metadata
	}{
		{
			name: "no meta",
			opts: PodDisruptionBudgetOpts{},
			wantMeta: &v1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
			opts: PodDisruptionBudgetOpts{
				Metadata: &v1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
			},
			wantMeta: &v1alpha1.Metadata{
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
			configMap, err := builder.BuildPodDisruptionBudget(tt.opts, &mariadbv1alpha1.MariaDB{})
			if err != nil {
				t.Fatalf("unexpected error building PDB: %v", err)
			}
			assertObjectMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
