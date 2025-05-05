package builder

import (
	"github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
)

func TestServiceMonitorMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *v1alpha1.Metadata
	}{
		{
			name: "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			wantMeta: &v1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
					InheritMetadata: &v1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
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
			svcMonitor, err := builder.BuildServiceMonitor(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building ServiceMonitor: %v", err)
			}
			assertObjectMeta(t, &svcMonitor.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
