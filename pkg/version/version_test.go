package version

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMinorVersion", func() {
	DescribeTable("returns the minor version",
		func(image, defaultVersion, wantMinorVersion string, wantErr bool) {
			var opts []Option
			if defaultVersion != "" {
				opts = append(opts, WithDefaultVersion(defaultVersion))
			}

			version, err := NewVersion(image, opts...)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if !wantErr {
				minorVersion, err := version.GetMinorVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(minorVersion).To(Equal(wantMinorVersion))
			}
		},
		Entry("empty", "", "", "", true),
		Entry("empty with default", "", "10.11.8", "10.11", false),
		Entry("invalid image", "10.11.8", "", "", true),
		Entry("invalid image with default", "10.11.8", "11.4", "11.4", false),
		Entry("non semver", "mariadb:latest", "", "", true),
		Entry("non semver with default", "mariadb:latest", "10.6", "10.6", false),
		Entry("sha256", "mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d", "", "", true),
		Entry("sha256 with default", "mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d", "11.4", "11.4", false),
		Entry("major", "mariadb:10", "", "10.0", false),
		Entry("major + minor", "mariadb:10.11", "", "10.11", false),
		Entry("major + minor + patch", "mariadb:10.11.8", "", "10.11", false),
		Entry("major + minor + patch + prerelease", "mariadb:10.11.8-ubi", "", "10.11", false),
		Entry("registry non semver", "registry-1.docker.io/v2/library/mariadb:latest", "", "", true),
		Entry("registry non semver with default", "registry-1.docker.io/v2/library/mariadb:latest", "10.6", "10.6", false),
		Entry("registry major + minor", "registry-1.docker.io/v2/library/mariadb:10.6", "", "10.6", false),
		Entry("registry major + minor + patch + prerelease", "registry-1.docker.io/v2/library/mariadb:10.6.18-14", "", "10.6", false),
		Entry(
			"registry sha256",
			"registry-1.docker.io/v2/library/mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"", "", true,
		),
		Entry(
			"registry sha256 with default",
			"registry-1.docker.io/v2/library/mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"10.6", "10.6", false,
		),
		Entry(
			"invalid default",
			"registry-1.docker.io/v2/library/mariadb@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"latest", "", true,
		),
	)
})

var _ = Describe("GreaterThanOrEqual", func() {
	DescribeTable("returns whether the version is greater than or equal to another",
		func(image, otherVersion string, wantBool, wantErr bool) {
			version, err := NewVersion(image)
			Expect(err).NotTo(HaveOccurred())

			gotBool, err := version.GreaterThanOrEqual(otherVersion)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(gotBool).To(Equal(wantBool))
		},
		Entry("empty", "mariadb:10.11.8", "", false, true),
		Entry("non semver", "mariadb:10.11.8", "latest", false, true),
		Entry("greater than", "mariadb:10.11.8", "10.6", true, false),
		Entry("greater than minor", "mariadb:10.11.8", "10.11", true, false),
		Entry("equal", "mariadb:10.11.8", "10.11.8", true, false),
		Entry("less than", "mariadb:10.11.8", "11.4.3", false, false),
	)
})
