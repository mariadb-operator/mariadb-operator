package client

import (
	"testing"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/galera/recovery"
)

func TestValidateClusterStateUUID(t *testing.T) {
	tests := []struct {
		name    string
		uuid    string
		wantErr bool
	}{
		{
			name:    "empty",
			uuid:    "",
			wantErr: true,
		},
		{
			name:    "zero uuid",
			uuid:    recovery.ZeroUUID,
			wantErr: true,
		},
		{
			name:    "valid uuid",
			uuid:    "f7f695b6-5000-11ef-8b0d-87e9e0e7b347",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClusterStateUUID(tt.uuid)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCoherentClusterStateUUID(t *testing.T) {
	valid := "f7f695b6-5000-11ef-8b0d-87e9e0e7b347"
	other := "8489ec18-246a-11f1-b580-d6b7feec7abb"

	tests := []struct {
		name    string
		uuids   []string
		want    string
		wantErr bool
	}{
		{
			name:    "empty set",
			uuids:   nil,
			wantErr: true,
		},
		{
			name:    "single valid",
			uuids:   []string{valid},
			want:    valid,
			wantErr: false,
		},
		{
			name:    "all matching",
			uuids:   []string{valid, valid, valid},
			want:    valid,
			wantErr: false,
		},
		{
			name:    "mismatch",
			uuids:   []string{valid, other},
			wantErr: true,
		},
		{
			name:    "zero uuid",
			uuids:   []string{recovery.ZeroUUID, recovery.ZeroUUID},
			wantErr: true,
		},
		{
			name:    "empty uuid",
			uuids:   []string{valid, ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CoherentClusterStateUUID(tt.uuids)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected uuid: got %s, want %s", got, tt.want)
			}
		})
	}
}
