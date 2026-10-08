package minio

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrefixedFileName", func() {
	DescribeTable("returns the prefixed S3 file name",
		func(client *Client, fileName string, wantFileName string) {
			Expect(client.PrefixedFileName(fileName)).To(Equal(wantFileName))
		},
		Entry("no prefix",
			&Client{},
			"backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("no prefix with file path",
			&Client{},
			"backup/backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"backup.2023-12-18T16:14:00Z.sql",
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix with file path",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"backup/backup.2023-12-18T16:14:00Z.sql",
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix with trailing slash",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			"backup.2023-12-18T16:14:00Z.sql",
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix with trailing slash and file path",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			"backup/backup.2023-12-18T16:14:00Z.sql",
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("nested prefix",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			"backup.2023-12-18T16:14:00Z.sql",
			"backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("nested prefix with file path",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			"backup/backup.2023-12-18T16:14:00Z.sql",
			"backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("already prefixed",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
		),
	)
})

var _ = Describe("UnprefixedFilename", func() {
	DescribeTable("returns the unprefixed S3 file name",
		func(client *Client, fileName string, wantFileName string) {
			Expect(client.UnprefixedFilename(fileName)).To(Equal(wantFileName))
		},
		Entry("no prefix",
			&Client{},
			"backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("no prefix with file path",
			&Client{},
			"backup/backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix with file path",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"backup/backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("prefix with trailing slash",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			"mariadb/backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("nested prefix",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			"backups/production/mariadb/backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
		Entry("already unprefixed",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"backup.2023-12-18T16:14:00Z.sql",
			"backup.2023-12-18T16:14:00Z.sql",
		),
	)
})

var _ = Describe("GetPrefix", func() {
	DescribeTable("returns the normalized S3 prefix",
		func(client *Client, wantPrefix string) {
			Expect(client.GetPrefix()).To(Equal(wantPrefix))
		},
		Entry("no prefix",
			&Client{},
			"",
		),
		Entry("root",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "/",
				},
			},
			"",
		),
		Entry("no trailing slash",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb",
				},
			},
			"mariadb/",
		),
		Entry("trailing slash",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "mariadb/",
				},
			},
			"mariadb/",
		),
		Entry("nested without trailing slash",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb",
				},
			},
			"backups/production/mariadb/",
		),
		Entry("nested with trailing slash",
			&Client{
				MinioOpts: MinioOpts{
					Prefix: "backups/production/mariadb/",
				},
			},
			"backups/production/mariadb/",
		),
	)
})

var _ = Describe("getSSEC", func() {
	// Valid 32-byte key for AES-256
	validKey := make([]byte, 32)
	for i := range validKey {
		validKey[i] = byte(i)
	}
	validKeyBase64 := base64.StdEncoding.EncodeToString(validKey)

	// Invalid key (not 32 bytes)
	invalidKey := make([]byte, 16)
	invalidKeyBase64 := base64.StdEncoding.EncodeToString(invalidKey)

	DescribeTable("returns the SSE-C configuration",
		func(client *Client, wantNil bool, wantErr bool) {
			sse, err := client.getSSEC()

			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(err).NotTo(HaveOccurred())

			if wantNil {
				Expect(sse).To(BeNil())
			} else {
				Expect(sse).NotTo(BeNil())
			}
		},
		Entry("no SSE-C key",
			&Client{},
			true,
			false,
		),
		Entry("empty SSE-C key",
			&Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: "",
				},
			},
			true,
			false,
		),
		Entry("valid SSE-C key",
			&Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: validKeyBase64,
				},
			},
			false,
			false,
		),
		Entry("invalid base64",
			&Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: invalidKeyBase64,
				},
			},
			true,
			true,
		),
		Entry("invalid base64 (not 32 bytes)",
			&Client{
				MinioOpts: MinioOpts{
					SSECCustomerKey: "not-valid-base64!!!",
				},
			},
			true,
			true,
		),
	)
})
