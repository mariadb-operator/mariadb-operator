package docker

import "testing"

// nolint:lll
func TestGetTag(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantTag string
		wantErr bool
	}{
		{
			name:    "invalid image",
			image:   "foo",
			wantTag: "",
			wantErr: true,
		},
		{
			name:    "empty tag",
			image:   "mariadb:",
			wantTag: "",
			wantErr: true,
		},
		{
			name:    "image",
			image:   "mariadb:10.6",
			wantTag: "10.6",
			wantErr: false,
		},
		{
			name:    "image with namespace",
			image:   "mariadb/maxscale:23.08.5",
			wantTag: "23.08.5",
			wantErr: false,
		},
		{
			name:    "image with namespace and host",
			image:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator:v0.0.1",
			wantTag: "v0.0.1",
			wantErr: false,
		},
		{
			name:    "digest",
			image:   "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			wantTag: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, err := GetTag(tt.image)
			if tt.wantTag != tag {
				t.Errorf("unexpected image, expected: %v got: %v", tt.wantTag, tag)
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

// nolint:lll
func TestGetDigest(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantDig string
		wantErr bool
	}{
		{
			name:    "invalid image",
			image:   "foo",
			wantDig: "",
			wantErr: true,
		},
		{
			name:    "empty digest",
			image:   "mariadb@",
			wantDig: "",
			wantErr: true,
		},
		{
			name:    "digest",
			image:   "docker.mariadb.com/enterprise-server@sha256:32ba72a21a2875b783887ecd4dcd7fd575a34cf253295e2bfa5ecd751545be37",
			wantDig: "sha256:32ba72a21a2875b783887ecd4dcd7fd575a34cf253295e2bfa5ecd751545be37",
			wantErr: false,
		},
		{
			name:    "tag",
			image:   "docker.mariadb.com/enterprise-server:11.8.3-1",
			wantDig: "",
			wantErr: true,
		},
		{
			name:    "image with host and digest",
			image:   "registry.mycorp.io/ns/img@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			wantDig: "sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dig, err := GetDigest(tt.image)
			if tt.wantDig != dig {
				t.Errorf("unexpected digest, expected: %v got: %v", tt.wantDig, dig)
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

// nolint:lll
func TestHasTagOrDigest(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{
			name:  "invalid image",
			image: "foo",
			want:  false,
		},
		{
			name:  "plain image",
			image: "mariadb",
			want:  false,
		},
		{
			name:  "tagged image",
			image: "docker.mariadb.com/enterprise-server:11.8.3-1",
			want:  true,
		},
		{
			name:  "digested image",
			image: "docker-registry3.mariadb.com/mariadb-enterprise-operator/mariadb-enterprise-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			want:  true,
		},
		{
			name:  "empty tag",
			image: "mariadb:",
			want:  false,
		},
		{
			name:  "empty digest",
			image: "mariadb@",
			want:  false,
		},
		{
			name:  "tag and digest",
			image: "registry.mycorp.io/ns/img:1.2@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := HasTagOrDigest(tt.image)
			if tt.want != ok {
				t.Errorf("unexpected result, expected: %v got: %v", tt.want, ok)
			}
		})
	}
}
