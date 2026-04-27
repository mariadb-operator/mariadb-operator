package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestServiceMonitorMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *mariadbv1alpha1.Metadata
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
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "with meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
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

func TestRoleRelabelConfig(t *testing.T) {
	tests := []struct {
		name            string
		podIndex        int
		primaryPodIndex *int
		wantRole        string
	}{
		{
			name:            "primary index is nil",
			podIndex:        0,
			primaryPodIndex: nil,
			wantRole:        "",
		},
		{
			name:            "pod is primary",
			podIndex:        0,
			primaryPodIndex: ptr.To(0),
			wantRole:        "primary",
		},
		{
			name:            "pod is replica",
			podIndex:        1,
			primaryPodIndex: ptr.To(0),
			wantRole:        "replica",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roleRelabelConfig := roleRelabelConfig(tt.podIndex, tt.primaryPodIndex)
			if tt.wantRole == "" {
				if roleRelabelConfig != nil {
					t.Errorf("expected empty relabel config, got %v", roleRelabelConfig)
				}
				return
			}
			if len(roleRelabelConfig) != 1 {
				t.Fatalf("expected 1 relabel config, got %d", len(roleRelabelConfig))
			}
			cfg := roleRelabelConfig[0]
			if cfg.Action != "replace" {
				t.Errorf("expected action 'replace', got %q", cfg.Action)
			}
			if cfg.TargetLabel != "role" {
				t.Errorf("expected target label 'role', got %q", cfg.TargetLabel)
			}
			if cfg.Replacement == nil || *cfg.Replacement != tt.wantRole {
				t.Errorf("expected role %q, got %v", tt.wantRole, cfg.Replacement)
			}
		})
	}
}
