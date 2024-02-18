package environment

import (
	"context"
	"reflect"
	"testing"
)

func TestWathcNamespaces(t *testing.T) {
	tests := []struct {
		name           string
		env            map[string]string
		wantNamespaces []string
		wantErr        bool
	}{
		{
			name:           "no env",
			env:            map[string]string{},
			wantNamespaces: nil,
			wantErr:        true,
		},
		{
			name: "all required but no WATCH_NAMESPACE",
			env: map[string]string{
				"MARIADB_OPERATOR_NAME":      "mariadb-operator",
				"MARIADB_OPERATOR_NAMESPACE": "mariadb-operator",
				"MARIADB_OPERATOR_SA_PATH":   "mariadb-operator",
				"MARIADB_OPERATOR_IMAGE":     "mariadb-operator",
				"RELATED_IMAGE_MARIADB":      "mariadb:lts",
				"RELATED_IMAGE_MAXSCALE":     "mariadb/maxscale:23.08",
				"RELATED_IMAGE_EXPORTER":     "prom/mysqld-exporter:v0.15.1",
			},
			wantNamespaces: nil,
			wantErr:        true,
		},
		{
			name: "all required but and empty WATCH_NAMESPACE",
			env: map[string]string{
				"MARIADB_OPERATOR_NAME":      "mariadb-operator",
				"MARIADB_OPERATOR_NAMESPACE": "mariadb-operator",
				"MARIADB_OPERATOR_SA_PATH":   "mariadb-operator",
				"MARIADB_OPERATOR_IMAGE":     "mariadb-operator",
				"RELATED_IMAGE_MARIADB":      "mariadb:lts",
				"RELATED_IMAGE_MAXSCALE":     "mariadb/maxscale:23.08",
				"RELATED_IMAGE_EXPORTER":     "prom/mysqld-exporter:v0.15.1",
				"WATCH_NAMESPACE":            "",
			},
			wantNamespaces: nil,
			wantErr:        true,
		},
		{
			name: "single namespace",
			env: map[string]string{
				"MARIADB_OPERATOR_NAME":      "mariadb-operator",
				"MARIADB_OPERATOR_NAMESPACE": "mariadb-operator",
				"MARIADB_OPERATOR_SA_PATH":   "mariadb-operator",
				"MARIADB_OPERATOR_IMAGE":     "mariadb-operator",
				"RELATED_IMAGE_MARIADB":      "mariadb:lts",
				"RELATED_IMAGE_MAXSCALE":     "mariadb/maxscale:23.08",
				"RELATED_IMAGE_EXPORTER":     "prom/mysqld-exporter:v0.15.1",
				"WATCH_NAMESPACE":            "ns1",
			},
			wantNamespaces: []string{"ns1"},
			wantErr:        false,
		},
		{
			name: "multiple namespaces",
			env: map[string]string{
				"MARIADB_OPERATOR_NAME":      "mariadb-operator",
				"MARIADB_OPERATOR_NAMESPACE": "mariadb-operator",
				"MARIADB_OPERATOR_SA_PATH":   "mariadb-operator",
				"MARIADB_OPERATOR_IMAGE":     "mariadb-operator",
				"RELATED_IMAGE_MARIADB":      "mariadb:lts",
				"RELATED_IMAGE_MAXSCALE":     "mariadb/maxscale:23.08",
				"RELATED_IMAGE_EXPORTER":     "prom/mysqld-exporter:v0.15.1",
				"WATCH_NAMESPACE":            "ns1,ns2,ns3",
			},
			wantNamespaces: []string{"ns1", "ns2", "ns3"},
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			env, err := GetOperatorEnv(context.Background())
			if err != nil && !tt.wantErr {
				t.Fatalf("unexpected error getting environment: %v", err)
			}
			if env == nil {
				return
			}

			namespaces, err := env.WatchNamespaces()
			if !reflect.DeepEqual(tt.wantNamespaces, namespaces) {
				t.Errorf("unexpected namespaces value: expected: %v, got: %v", tt.wantNamespaces, env)
			}
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}
