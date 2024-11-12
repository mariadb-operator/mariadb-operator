package version

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetMinorVersion(t *testing.T) {
	tests := []struct {
		name             string
		image            string
		defaultVersion   string
		wantMinorVersion string
		wantErr          bool
	}{
		{
			name:             "empty",
			image:            "",
			defaultVersion:   "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "empty with default",
			image:            "",
			defaultVersion:   "10.11.8",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "invalid image",
			image:            "10.11.8",
			defaultVersion:   "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "invalid image with default",
			image:            "10.11.8",
			defaultVersion:   "11.4",
			wantMinorVersion: "11.4",
			wantErr:          false,
		},
		{
			name:             "non semver",
			image:            "mariadb:latest",
			defaultVersion:   "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "non semver with default",
			image:            "mariadb:latest",
			defaultVersion:   "10.6",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
		{
			name:             "sha256",
			image:            "mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			defaultVersion:   "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "sha256 with default",
			image:            "mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			defaultVersion:   "11.4",
			wantMinorVersion: "11.4",
			wantErr:          false,
		},
		{
			name:             "major",
			image:            "mariadb:10",
			defaultVersion:   "",
			wantMinorVersion: "10.0",
			wantErr:          false,
		},
		{
			name:             "major + minor",
			image:            "mariadb:10.11",
			defaultVersion:   "",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "major + minor + patch",
			image:            "mariadb:10.11.8",
			defaultVersion:   "",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "major + minor + patch + prerelease",
			image:            "mariadb:10.11.8-ubi",
			defaultVersion:   "",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "enterprise non semver",
			image:            "docker.mariadb.com/enterprise-server:latest",
			defaultVersion:   "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "enterprise non semver with default",
			image:            "docker.mariadb.com/enterprise-server:latest",
			defaultVersion:   "10.6",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
		{
			name:             "enterprise major + minor",
			image:            "docker.mariadb.com/enterprise-server:10.6",
			defaultVersion:   "",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
		{
			name:             "enterprise major + minor + patch + prerelease",
			image:            "docker.mariadb.com/enterprise-server:10.6.18-14",
			defaultVersion:   "",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
		{
			name:             "enterprise sha256",
			image:            "docker.mariadb.com/enterprise-server@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			defaultVersion:   "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "enterprise sha256 with default",
			image:            "docker.mariadb.com/enterprise-server@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			defaultVersion:   "10.6",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
		{
			name:             "invalid default",
			image:            "docker.mariadb.com/enterprise-server@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			defaultVersion:   "latest",
			wantMinorVersion: "",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.defaultVersion != "" {
				opts = append(opts, WithDefaultVersion(tt.defaultVersion))
			}

			version, err := NewVersion(tt.image, opts...)
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error creating version: %v", err)
			}

			if !tt.wantErr {
				minorVersion, err := version.GetMinorVersion()
				if err != nil {
					t.Errorf("unexpected error getting minor version: %v", err)
				}
				if diff := cmp.Diff(tt.wantMinorVersion, minorVersion); diff != "" {
					t.Errorf("unexpected minor version (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestGreaterThanOrEqual(t *testing.T) {
	tests := []struct {
		name         string
		image        string
		otherVersion string
		wantBool     bool
		wantErr      bool
	}{
		{
			name:         "empty",
			image:        "mariadb:10.11.8",
			otherVersion: "",
			wantBool:     false,
			wantErr:      true,
		},
		{
			name:         "non semver",
			image:        "mariadb:10.11.8",
			otherVersion: "latest",
			wantBool:     false,
			wantErr:      true,
		},
		{
			name:         "greater than",
			image:        "mariadb:10.11.8",
			otherVersion: "10.6",
			wantBool:     true,
			wantErr:      false,
		},
		{
			name:         "greater than minor",
			image:        "mariadb:10.11.8",
			otherVersion: "10.11",
			wantBool:     true,
			wantErr:      false,
		},
		{
			name:         "equal",
			image:        "mariadb:10.11.8",
			otherVersion: "10.11.8",
			wantBool:     true,
			wantErr:      false,
		},
		{
			name:         "less than",
			image:        "mariadb:10.11.8",
			otherVersion: "11.4.3",
			wantBool:     false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := NewVersion(tt.image)
			if err != nil {
				t.Errorf("unexpected error creating version: %v", err)
			}

			gotBool, err := version.GreaterThanOrEqual(tt.otherVersion)
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error checking version: %v", err)
			}

			if diff := cmp.Diff(tt.wantBool, gotBool); diff != "" {
				t.Errorf("unexpected bool (-want +got):\n%s", diff)
			}
		})
	}
}
