package replication

import (
	"testing"
)

func TestDemoteToSlaveSupported(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		want    bool
		wantErr bool
	}{
		{
			name:  "mariadb 11.x supports demote",
			image: "mariadb:11.8.8",
			want:  true,
		},
		{
			name:  "mariadb 10.11 supports demote",
			image: "mariadb:10.11.6",
			want:  true,
		},
		{
			name:  "mariadb 10.10 supports demote",
			image: "mariadb:10.10.3",
			want:  true,
		},
		{
			name:  "mariadb 10.9 does not support demote",
			image: "mariadb:10.9.13",
			want:  false,
		},
		{
			name:  "mariadb 10.5 does not support demote",
			image: "mariadb:10.5.22",
			want:  false,
		},
		{
			name:    "sha256 digest cannot be inferred",
			image:   "mariadb@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:    false,
			wantErr: true,
		},
		{
			name:    "image without tag cannot be inferred",
			image:   "mariadb",
			want:    false,
			wantErr: true,
		},
		{
			name:    "non-semver tag cannot be inferred",
			image:   "mariadb:latest",
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := demoteToSlaveSupported(tt.image)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
