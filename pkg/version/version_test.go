package version

import "testing"

func TestGetMinorVersion(t *testing.T) {
	tests := []struct {
		name             string
		image            string
		wantMinorVersion string
		wantErr          bool
	}{
		{
			name:             "empty",
			image:            "",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "invalid image",
			image:            "10.11.8",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "latest",
			image:            "mariadb:latest",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "lts",
			image:            "mariadb:lts-noble",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "major",
			image:            "mariadb:10",
			wantMinorVersion: "10.0",
			wantErr:          false,
		},
		{
			name:             "major + minor",
			image:            "mariadb:10.11",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "major + minor + patch",
			image:            "mariadb:10.11.8",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "major + minor + patch + prerelease",
			image:            "mariadb:10.11.8-ubi",
			wantMinorVersion: "10.11",
			wantErr:          false,
		},
		{
			name:             "enterprise: latest",
			image:            "docker.mariadb.com/enterprise-server:latest",
			wantMinorVersion: "",
			wantErr:          true,
		},
		{
			name:             "enterprise: major + minor",
			image:            "docker.mariadb.com/enterprise-server:10.6",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
		{
			name:             "enterprise: major + minor + patch + prerelease",
			image:            "docker.mariadb.com/enterprise-server:10.6.18-14",
			wantMinorVersion: "10.6",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minorVersion, err := GetMinorVersion(tt.image)
			if tt.wantMinorVersion != minorVersion {
				t.Errorf("want minor version \"%s\", got: \"%s\"", tt.wantMinorVersion, minorVersion)
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
