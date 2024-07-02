package embed

import (
	"context"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
)

func TestReadEntrypoint(t *testing.T) {
	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		env       *environment.OperatorEnv
		wantBytes bool
		wantErr   bool
	}{
		{
			name:      "empty",
			mariadb:   &mariadbv1alpha1.MariaDB{},
			env:       &environment.OperatorEnv{},
			wantBytes: false,
			wantErr:   true,
		},
		{
			name: "invalid version",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:foo",
				},
			},
			env:       &environment.OperatorEnv{},
			wantBytes: false,
			wantErr:   true,
		},
		{
			name: "default invalid version",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:foo",
				},
			},
			env: &environment.OperatorEnv{
				MariadbEntrypointVersion: "10.11",
			},
			wantBytes: true,
			wantErr:   false,
		},
		{
			name: "unsupported version",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:8.0.0",
				},
			},
			env:       &environment.OperatorEnv{},
			wantBytes: false,
			wantErr:   true,
		},
		{
			name: "default unsupported version",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:8.0.0",
				},
			},
			env: &environment.OperatorEnv{
				MariadbEntrypointVersion: "10.11",
			},
			wantBytes: true,
			wantErr:   false,
		},
		{
			name: "supported version",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "mariadb:10.11.8",
				},
			},
			wantBytes: true,
			wantErr:   false,
		},
		{
			name: "supported enterprise version",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Image: "docker.mariadb.com/enterprise-server:10.6.18-14",
				},
			},
			wantBytes: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := ReadEntrypoint(context.Background(), tt.mariadb, tt.env)
			if tt.wantBytes && bytes == nil {
				t.Error("expected bytes but got nil")
			}
			if !tt.wantBytes && bytes != nil {
				t.Error("unexpected bytes")
			}
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error getting minor version: %v", err)
			}
		})
	}
}
