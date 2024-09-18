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
