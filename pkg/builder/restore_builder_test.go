package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRestoreMeta(t *testing.T) {
	builder := newTestBuilder()
	key := types.NamespacedName{
		Name: "restore",
	}
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
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
		{
			name:    "meta",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap, err := builder.BuildRestore(tt.mariadb, key)
			if err != nil {
				t.Fatalf("unexpected error building Restore: %v", err)
			}
			assertMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
