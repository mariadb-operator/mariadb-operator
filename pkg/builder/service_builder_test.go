package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func TestServiceMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
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
				ExtraMeta:             &mariadbv1alpha1.Metadata{},
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
				ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
					Metadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": "172.18.0.20",
						},
					},
				},
				ExcludeSelectorLabels: true,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"metallb.universe.tf/loadBalancerIPs": "172.18.0.20",
				},
			},
		},
		{
			name: "extra meta",
			opts: ServiceOpts{
				ExtraMeta: &mariadbv1alpha1.Metadata{
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
		{
			name: "meta and extra meta",
			opts: ServiceOpts{
				ServiceTemplate: mariadbv1alpha1.ServiceTemplate{
					Metadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"metallb.universe.tf/loadBalancerIPs": "172.18.0.20",
						},
					},
				},
				ExtraMeta: &mariadbv1alpha1.Metadata{
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
					"database.myorg.io":                   "mariadb",
					"metallb.universe.tf/loadBalancerIPs": "172.18.0.20",
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
			assertObjectMeta(t, &configMap.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
