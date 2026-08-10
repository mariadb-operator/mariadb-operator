package compression

import (
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/backup"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BackupCompressors", func() {
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	processor := backup.NewLogicalBackupProcessor()
	logger := logr.Discard()

	DescribeTable("compresses and decompresses backup files",
		//nolint:lll
		func(newCompressorFn func(basePath string, getUncompressedFilename GetBackupUncompressedFilenameFn, logger logr.Logger) BackupCompressor, fileName string) {
			dir, err := os.MkdirTemp("", "backup_test")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, dir)

			compressor := newCompressorFn(dir, processor.GetUncompressedBackupFile, logger)

			filePath := filepath.Join(dir, fileName)
			Expect(os.WriteFile(filePath, []byte(content), 0644)).To(Succeed())

			Expect(compressor.Compress(filePath)).To(Succeed())
			decompressedFileName, err := compressor.Decompress(filePath)
			Expect(err).NotTo(HaveOccurred())

			decompressedContent, err := os.ReadFile(decompressedFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(decompressedContent)).To(Equal(content))
		},
		Entry("nop", NewNopBackupCompressor, "backup.2023-12-18T16:14:00Z.sql"),
		Entry("gzip", NewGzipBackupCompressor, "backup.2023-12-18T16:14:00Z.sql.gz"),
		Entry("bzip2", NewBzip2BackupCompressor, "backup.2023-12-18T16:14:00Z.sql.bz2"),
	)
})
