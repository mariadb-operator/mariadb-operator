package environment

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWatchNamespaces(t *testing.T) {
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
			name: "single namespace",
			env: map[string]string{
				"WATCH_NAMESPACE": "ns1",
			},
			wantNamespaces: []string{"ns1"},
			wantErr:        false,
		},
		{
			name: "multiple namespaces",
			env: map[string]string{
				"WATCH_NAMESPACE": "ns1,ns2,ns3",
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

func TestCurrentNamespaceOnly(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantBool bool
	}{
		{
			name:     "no env",
			env:      map[string]string{},
			wantBool: false,
		},
		{
			name: "same namespace",
			env: map[string]string{
				"WATCH_NAMESPACE":            "ns1",
				"MARIADB_OPERATOR_NAMESPACE": "ns1",
			},
			wantBool: true,
		},
		{
			name: "other namespace",
			env: map[string]string{
				"WATCH_NAMESPACE":            "ns2",
				"MARIADB_OPERATOR_NAMESPACE": "ns1",
			},
			wantBool: false,
		},
		{
			name: "multiple namespaces",
			env: map[string]string{
				"WATCH_NAMESPACE":            "ns1,ns2,ns3",
				"MARIADB_OPERATOR_NAMESPACE": "ns1",
			},
			wantBool: false,
		},
		{
			name: "all namespaces",
			env: map[string]string{
				"WATCH_NAMESPACE":            "",
				"MARIADB_OPERATOR_NAMESPACE": "ns1",
			},
			wantBool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			env, err := GetOperatorEnv(context.Background())
			if err != nil {
				t.Fatalf("unexpected error getting environment: %v", err)
			}
			if env == nil {
				return
			}

			currentNamespaceOnly, err := env.CurrentNamespaceOnly()
			if !reflect.DeepEqual(tt.wantBool, currentNamespaceOnly) {
				t.Errorf("unexpected currentNamespaceOnly value: expected: %v, got: %v", tt.wantBool, env)
			}
			if err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestTLSEnabled(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantBool bool
		wantErr  bool
	}{
		{
			name:     "no env",
			env:      map[string]string{},
			wantBool: false,
			wantErr:  false,
		},
		{
			name: "empty",
			env: map[string]string{
				"TLS_ENABLED": "",
			},
			wantBool: false,
			wantErr:  false,
		},
		{
			name: "invalid",
			env: map[string]string{
				"TLS_ENABLED": "foo",
			},
			wantBool: false,
			wantErr:  true,
		},
		{
			name: "valid bool",
			env: map[string]string{
				"TLS_ENABLED": "true",
			},
			wantBool: true,
			wantErr:  false,
		},
		{
			name: "valid number",
			env: map[string]string{
				"TLS_ENABLED": "1",
			},
			wantBool: true,
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLUSTER_NAME", "test")
			t.Setenv("POD_NAME", "mariadb-0")
			t.Setenv("POD_NAMESPACE", "default")
			t.Setenv("POD_IP", "10.244.0.11")
			t.Setenv("MARIADB_NAME", "mariadb")
			t.Setenv("MARIADB_ROOT_PASSWORD", "MariaDB11!")
			t.Setenv("MYSQL_TCP_PORT", "3306")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			env, err := GetPodEnv(context.Background())
			if err != nil {
				t.Fatalf("unexpected error getting environment: %v", err)
			}
			if env == nil {
				return
			}

			isTLSEnabled, err := env.IsTLSEnabled()
			gotErr := err != nil
			if diff := cmp.Diff(tt.wantErr, gotErr); diff != "" {
				t.Errorf("unexpected err (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantBool, isTLSEnabled); diff != "" {
				t.Errorf("unexpected bool (-want +got):\n%s", diff)
			}
		})
	}
}
