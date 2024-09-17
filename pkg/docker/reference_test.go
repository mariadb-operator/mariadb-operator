package docker

import "testing"

// nolint:lll
func TestSetTagOrDigest(t *testing.T) {
	tests := []struct {
		name        string
		sourceImage string
		targetImage string
		wantImage   string
		wantErr     bool
	}{
		{
			name:        "invalid source",
			sourceImage: "foo",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "invalid source tag",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "invalid source digest",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:foo",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "invalid target",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			targetImage: "foo",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "invalid target tag",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "invalid target digest",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:foo",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "no tag nor digest in source",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "",
			wantErr:     true,
		},
		{
			name:        "no tag nor digest in target",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator",
			wantImage:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantErr:     false,
		},
		{
			name:        "tag source, tag target",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			wantErr:     false,
		},
		{
			name:        "digest source, tag target",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			wantErr:     false,
		},
		{
			name:        "tag source, digest target",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			wantImage:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			wantErr:     false,
		},
		{
			name:        "digest source, digest target",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			targetImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			wantImage:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			wantErr:     false,
		},
		{
			name:        "different host",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			targetImage: "registry.mycorp.io/mariadb-operator/mariadb-operator:v0.0.1",
			wantImage:   "registry.mycorp.io/mariadb-operator/mariadb-operator:v0.0.2",
			wantErr:     false,
		},
		{
			name:        "different host, namespace and image",
			sourceImage: "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.2",
			targetImage: "registry.mycorp.io/mdb-op/mdb-op:v0.0.1",
			wantImage:   "registry.mycorp.io/mdb-op/mdb-op:v0.0.2",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := SetTagOrDigest(tt.sourceImage, tt.targetImage)
			if tt.wantImage != image {
				t.Errorf("unexpected image, expected: %v got: %v", tt.wantImage, image)
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
