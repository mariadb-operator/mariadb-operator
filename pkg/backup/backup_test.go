package backup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetFilePath", func() {
	DescribeTable("returns the file path",
		func(path, fileName, wantFilePath string) {
			filePath := GetFilePath(path, fileName)
			Expect(filePath).To(Equal(wantFilePath))
		},
		Entry("empty path",
			"",
			"backup.2023-12-22T13:00:00Z.foo.sql",
			"backup.2023-12-22T13:00:00Z.foo.sql",
		),
		Entry("add path",
			"/backup",
			"backup.2023-12-22T13:00:00Z.foo.sql",
			"/backup/backup.2023-12-22T13:00:00Z.foo.sql",
		),
		Entry("already has relative path",
			"/backup",
			"mariadb/backup.2023-12-22T13:00:00Z.foo.sql",
			"/backup/mariadb/backup.2023-12-22T13:00:00Z.foo.sql",
		),
		Entry("already has absolute path",
			"/backup",
			"/backup/backup.2023-12-22T13:00:00Z.foo.sql",
			"/backup/backup.2023-12-22T13:00:00Z.foo.sql",
		),
	)
})
