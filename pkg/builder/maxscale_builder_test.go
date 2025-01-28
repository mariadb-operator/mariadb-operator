package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestMaxScaleMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "maxscale",
	}
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name:    "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mxs, err := builder.BuildMaxScale(key, tt.mariadb, &mariadbv1alpha1.MariaDBMaxScaleSpec{})
			if err != nil {
				t.Fatalf("unexpected error building MaxScale: %v", err)
			}
			assertObjectMeta(t, &mxs.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestMaxScaleTLS(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "maxscale",
	}
	tests := []struct {
		name    string
		mariadb *mariadbv1alpha1.MariaDB
		mdbmxs  *mariadbv1alpha1.MariaDBMaxScaleSpec
		wantTLS *mariadbv1alpha1.MaxScaleTLS
	}{
		{
			name: "tls not enabled in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: false,
					},
				},
			},
			mdbmxs:  &mariadbv1alpha1.MariaDBMaxScaleSpec{},
			wantTLS: nil,
		},
		{
			name: "tls enabled in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			mdbmxs:  &mariadbv1alpha1.MariaDBMaxScaleSpec{},
			wantTLS: nil,
		},
		{
			name: "tls required in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled:  true,
						Required: ptr.To(true),
					},
				},
			},
			mdbmxs:  &mariadbv1alpha1.MariaDBMaxScaleSpec{},
			wantTLS: &mariadbv1alpha1.MaxScaleTLS{Enabled: true},
		},
		{
			name: "tls explicitly set in MaxScaleSpec",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{},
			},
			mdbmxs: &mariadbv1alpha1.MariaDBMaxScaleSpec{
				TLS: &mariadbv1alpha1.MaxScaleTLS{Enabled: true},
			},
			wantTLS: &mariadbv1alpha1.MaxScaleTLS{Enabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mxs, err := builder.BuildMaxScale(key, tt.mariadb, tt.mdbmxs)
			if err != nil {
				t.Fatalf("unexpected error building MaxScale: %v", err)
			}
			if tt.wantTLS == nil {
				if mxs.Spec.TLS != nil {
					t.Errorf("expected TLS to be nil, got %v", mxs.Spec.TLS)
				}
			} else {
				if mxs.Spec.TLS == nil {
					t.Errorf("expected TLS to be %v, got nil", tt.wantTLS)
				} else if mxs.Spec.TLS.Enabled != tt.wantTLS.Enabled {
					t.Errorf("expected TLS enabled to be %v, got %v", tt.wantTLS.Enabled, mxs.Spec.TLS.Enabled)
				}
			}
		})
	}
}
