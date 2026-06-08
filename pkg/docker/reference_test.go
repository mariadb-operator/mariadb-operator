package docker

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetTag", func() {
	// nolint:lll
	DescribeTable("returns the tag of an image",
		func(image string, wantTag string, wantErr bool) {
			tag, err := GetTag(image)
			Expect(tag).To(Equal(wantTag))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("invalid image", "foo", "", true),
		Entry("empty tag", "mariadb:", "", true),
		Entry("image", "mariadb:10.6", "10.6", false),
		Entry("image with namespace", "mariadb/maxscale:23.08.5", "23.08.5", false),
		Entry("image with namespace and host",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1", "v0.0.1", false),
		Entry("digest",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"", true),
	)
})

var _ = Describe("SetTagOrDigest", func() {
	// nolint:lll
	DescribeTable("sets the tag or digest of the target image from the source image",
		func(sourceImage string, targetImage string, wantImage string, wantErr bool) {
			image, err := SetTagOrDigest(sourceImage, targetImage)
			Expect(image).To(Equal(wantImage))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("invalid source",
			"foo",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"", true),
		Entry("invalid source tag",
			"ghcr.io/mariadb-operator/mariadb-operator:",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"", true),
		Entry("invalid source digest",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:foo",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"", true),
		Entry("invalid target",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"foo",
			"", true),
		Entry("invalid target tag",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"ghcr.io/mariadb-operator/mariadb-operator:",
			"", true),
		Entry("invalid target digest",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:foo",
			"", true),
		Entry("no tag nor digest in source",
			"ghcr.io/mariadb-operator/mariadb-operator",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"", true),
		Entry("no tag nor digest in target",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"ghcr.io/mariadb-operator/mariadb-operator",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1", false),
		Entry("tag source, tag target",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.2",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.2", false),
		Entry("digest source, tag target",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.1",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			false),
		Entry("tag source, digest target",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.2",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.2", false),
		Entry("digest source, digest target",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			"ghcr.io/mariadb-operator/mariadb-operator@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			false),
		Entry("different host",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.2",
			"registry.mycorp.io/mariadb-operator/mariadb-operator:v0.0.1",
			"registry.mycorp.io/mariadb-operator/mariadb-operator:v0.0.2", false),
		Entry("different host, namespace and image",
			"ghcr.io/mariadb-operator/mariadb-operator:v0.0.2",
			"registry.mycorp.io/mdb-op/mdb-op:v0.0.1",
			"registry.mycorp.io/mdb-op/mdb-op:v0.0.2", false),
	)
})

var _ = Describe("GetDigest", func() {
	// nolint:lll
	DescribeTable("returns the digest of an image",
		func(image string, wantDig string, wantErr bool) {
			dig, err := GetDigest(image)
			Expect(dig).To(Equal(wantDig))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("invalid image", "foo", "", true),
		Entry("empty digest", "mariadb@", "", true),
		Entry("digest",
			"docker.mariadb.com/enterprise-server@sha256:32ba72a21a2875b783887ecd4dcd7fd575a34cf253295e2bfa5ecd751545be37",
			"sha256:32ba72a21a2875b783887ecd4dcd7fd575a34cf253295e2bfa5ecd751545be37", false),
		Entry("tag", "docker.mariadb.com/enterprise-server:11.8.3-1", "", true),
		Entry("image with host and digest",
			"registry.mycorp.io/ns/img@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			"sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3", false),
	)
})

var _ = Describe("HasTagOrDigest", func() {
	// nolint:lll
	DescribeTable("returns whether an image has a tag or digest",
		func(image string, want bool) {
			ok := HasTagOrDigest(image)
			Expect(ok).To(Equal(want))
		},
		Entry("invalid image", "foo", false),
		Entry("plain image", "mariadb", false),
		Entry("tagged image", "docker.mariadb.com/enterprise-server:11.8.3-1", true),
		Entry("digested image",
			"ghcr.io/mariadb-enterprise-operator/mariadb-enterprise-operator@sha256:3f48454b6a33e094af6d23ced54645ec0533cb11854d07738920852ca48e390d",
			true),
		Entry("empty tag", "mariadb:", false),
		Entry("empty digest", "mariadb@", false),
		Entry("tag and digest",
			"registry.mycorp.io/ns/img:1.2@sha256:5e3d39d26829673c7b4f6f21fb1d57902c9bb64367a76dc2b74e5909027f25a3",
			true),
	)
})
