package backup

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	mdbtime "github.com/mariadb-operator/mariadb-operator/v26/pkg/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var logger = ctrl.Log.WithName("test")

var _ = Describe("LogicalGetTargetFile", func() {
	p := NewLogicalBackupProcessor()
	DescribeTable("returns the backup target file",
		func(backupFiles []string, targetRecovery time.Time, wantFile string, wantErr bool) {
			file, err := p.GetBackupTargetFile(backupFiles, targetRecovery, logger)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(file).To(Equal(wantFile))
		},
		Entry("no backups",
			[]string{},
			time.Now(),
			"",
			true,
		),
		Entry("invalid backups",
			[]string{
				"backup.foo.sql",
				"backup.bar.sql",
				"backup.sql",
			},
			time.Now(),
			"",
			true,
		),
		Entry("single backup",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
			},
			time.Now(),
			"backup.2023-12-18T15:58:00Z.sql",
			false,
		),
		Entry("multiple backups with invalid",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.foo.sql",
				"backup.foo.sql",
			},
			mustParseDate("2023-12-18T15:59:00Z"),
			"backup.2023-12-18T15:58:00Z.sql",
			false,
		),
		Entry("fine grained",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:58:01Z.sql",
				"backup.2023-12-18T16:00:Z.sql",
			},
			mustParseDate("2023-12-18T15:59:00Z"),
			"backup.2023-12-18T15:58:01Z.sql",
			false,
		),
		Entry("target before backups",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			time.UnixMilli(0),
			"",
			true,
		),
		Entry("target after backups",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			time.Now(),
			"backup.2023-12-18T16:13:00Z.sql",
			false,
		),
		Entry("close target",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			mustParseDate("2023-12-18T16:04:00Z"),
			"backup.2023-12-18T16:03:00Z.sql",
			false,
		),
		Entry("exact target",
			[]string{
				"backup.2023-12-18T15:58:00Z.sql",
				"backup.2023-12-18T15:59:00Z.sql",
				"backup.2023-12-18T16:00:00Z.sql",
				"backup.2023-12-18T16:03:00Z.sql",
				"backup.2023-12-18T16:07:00Z.sql",
				"backup.2023-12-18T16:08:00Z.sql",
				"backup.2023-12-18T16:09:00Z.sql",
				"backup.2023-12-18T16:12:00Z.sql",
				"backup.2023-12-18T16:13:00Z.sql",
			},
			mustParseDate("2023-12-18T16:07:00Z"),
			"backup.2023-12-18T16:07:00Z.sql",
			false,
		),
	)
})

var _ = Describe("LogicalGetOldBackupFiles", func() {
	p := NewLogicalBackupProcessor()
	DescribeTable("returns the old backup files",
		func(nowFn func() time.Time, backupFiles []string, maxRetention time.Duration, wantBackups []string) {
			previousNowFn := now
			DeferCleanup(func() {
				now = previousNowFn
			})
			now = nowFn

			backups := p.GetOldBackupFiles(backupFiles, maxRetention, logger)
			Expect(backups).To(Equal(wantBackups))
		},
		Entry("no backups",
			testTimeFn(mustParseDate("2023-12-22T22:10:00Z")),
			nil,
			1*time.Hour,
			nil,
		),
		Entry("invalid backups",
			testTimeFn(mustParseDate("2023-12-22T22:10:00Z")),
			[]string{
				"backup.foo.sql",
				"backup.bar.sql",
				"backup.sql",
			},
			1*time.Hour,
			nil,
		),
		Entry("no old backups",
			testTimeFn(mustParseDate("2023-12-22T22:10:00Z")),
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
			24*time.Hour,
			nil,
		),
		Entry("multiple old backups",
			testTimeFn(mustParseDate("2023-12-22T22:10:00Z")),
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
			8*time.Hour,
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
			},
		),
		Entry("multiple old backups with invalid",
			testTimeFn(mustParseDate("2023-12-22T22:10:00Z")),
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
				"backup.foo.sql",
				"backup.bar.sql",
				"backup.sql",
			},
			8*time.Hour,
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
			},
		),
		Entry("all old backups",
			testTimeFn(mustParseDate("2023-12-22T22:10:00Z")),
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
			1*time.Hour,
			[]string{
				"backup.2023-12-22T13:00:00Z.sql",
				"backup.2023-12-22T14:00:00Z.sql",
				"backup.2023-12-22T15:00:00Z.sql",
				"backup.2023-12-22T16:00:00Z.sql",
				"backup.2023-12-22T17:00:00Z.sql",
				"backup.2023-12-22T18:00:00Z.sql",
				"backup.2023-12-22T19:00:00Z.sql",
				"backup.2023-12-22T20:00:00Z.sql",
			},
		),
	)
})

var _ = Describe("LogicalIsValidBackupFile", func() {
	p := NewLogicalBackupProcessor()
	DescribeTable("returns whether the backup file is valid",
		func(backupFile string, wantValid bool) {
			valid := p.IsValidBackupFile(backupFile)
			Expect(valid).To(Equal(wantValid))
		},
		Entry("empty", "", false),
		Entry("no date", "backup.sql", false),
		Entry("no prefix", "2023-12-18 16:14.sql", false),
		Entry("no extension", "backup.2023-12-18T16:14:00Z", false),
		Entry("invalid date", "backup.2023-12-18 16:14.sql", false),
		Entry("invalid compression", "backup.2023-12-18 16:14.foo.sql", false),
		Entry("valid", "backup.2023-12-18T16:14:00Z.sql", true),
		Entry("valid with legacy compression", "backup.2023-12-18T16:14:00Z.bzip2.sql", true),
		Entry("valid with gzip compression", "backup.2023-12-18T16:14:00Z.sql.gz", true),
		Entry("valid with bzip2 compression", "backup.2023-12-18T16:14:00Z.sql.bz2", true),
	)
})

var _ = Describe("LogicalParseCompressionAlgorithm", func() {
	p := NewLogicalBackupProcessor()
	DescribeTable("parses the compression algorithm",
		func(fileName string, wantCompress mariadbv1alpha1.CompressAlgorithm, wantErr bool) {
			compress, err := p.ParseCompressionAlgorithm(fileName)
			Expect(compress).To(Equal(wantCompress))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", "", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("invalid", "foo", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("invalid format", "backup.sql", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("no compression", "backup.2023-12-22T13:00:00Z.sql", mariadbv1alpha1.CompressNone, false),
		Entry("invalid compression", "backup.2023-12-22T13:00:00Z.foo.sql", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("legacy compression gzip", "backup.2023-12-22T13:00:00Z.gzip.sql", mariadbv1alpha1.CompressGzip, false),
		Entry("legacy compression bzip2", "backup.2023-12-22T13:00:00Z.bzip2.sql", mariadbv1alpha1.CompressBzip2, false),
		Entry("new format compression gz", "backup.2023-12-22T13:00:00Z.sql.gz", mariadbv1alpha1.CompressGzip, false),
		Entry("new format compression bz2", "backup.2023-12-22T13:00:00Z.sql.bz2", mariadbv1alpha1.CompressBzip2, false),
		Entry("new format invalid extension", "backup.2023-12-22T13:00:00Z.sql.foo", mariadbv1alpha1.CompressAlgorithm(""), true),
	)
})

var _ = Describe("LogicalGetUncompressedBackupFile", func() {
	p := NewLogicalBackupProcessor()
	DescribeTable("returns the uncompressed backup file",
		func(fileName, wantFileName string, wantErr bool) {
			fileName, err := p.GetUncompressedBackupFile(fileName)
			Expect(fileName).To(Equal(wantFileName))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", "", "", true),
		Entry("invalid", "foo", "", true),
		Entry("invalid format", "backup.sql", "", true),
		Entry("no compression", "backup.2023-12-22T13:00:00Z.sql", "", true),
		Entry("invalid compression", "backup.2023-12-22T13:00:00Z.foo.sql", "", true),
		Entry("legacy compression gzip", "backup.2023-12-22T13:00:00Z.gzip.sql", "backup.2023-12-22T13:00:00Z.sql", false),
		Entry("legacy compression bzip2", "backup.2023-12-22T13:00:00Z.bzip2.sql", "backup.2023-12-22T13:00:00Z.sql", false),
		Entry("new format compression gz", "backup.2023-12-22T13:00:00Z.sql.gz", "backup.2023-12-22T13:00:00Z.sql", false),
		Entry("new format compression bz2", "backup.2023-12-22T13:00:00Z.sql.bz2", "backup.2023-12-22T13:00:00Z.sql", false),
		Entry("new format invalid extension", "backup.2023-12-22T13:00:00Z.sql.foo", "", true),
	)
})

var _ = Describe("PhysicalGetTargetFile", func() {
	p := NewPhysicalBackupProcessor()
	DescribeTable("returns the backup target file",
		func(backupFiles []string, targetRecovery time.Time, wantFile string, wantErr bool) {
			file, err := p.GetBackupTargetFile(backupFiles, targetRecovery, logger)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(file).To(Equal(wantFile))
		},
		Entry("no backups",
			[]string{},
			time.Now(),
			"",
			true,
		),
		Entry("invalid backups",
			[]string{
				"physicalbackup.foo.xb",
				"physicalbackup.bar.xb",
				"physicalbackup.xb",
			},
			time.Now(),
			"",
			true,
		),
		Entry("single backup",
			[]string{
				"physicalbackup-20231218155800.xb",
			},
			time.Now(),
			"physicalbackup-20231218155800.xb",
			false,
		),
		Entry("multiple backups with invalid",
			[]string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup.foo.xb",
				"physicalbackup.bar.xb",
			},
			mustParseMariadbDate("20231218155900"),
			"physicalbackup-20231218155800.xb",
			false,
		),
		Entry("fine grained",
			[]string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218155801.xb",
				"physicalbackup-20231218160000.xb",
			},
			mustParseMariadbDate("20231218155900"),
			"physicalbackup-20231218155801.xb",
			false,
		),
		Entry("target before backups",
			[]string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218155900.xb",
			},
			time.UnixMilli(0),
			"",
			true,
		),
		Entry("target after backups",
			[]string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218161300.xb",
			},
			time.Now(),
			"physicalbackup-20231218161300.xb",
			false,
		),
		Entry("exact target",
			[]string{
				"physicalbackup-20231218155800.xb",
				"physicalbackup-20231218160700.xb",
			},
			mustParseMariadbDate("20231218160700"),
			"physicalbackup-20231218160700.xb",
			false,
		),
		Entry("prefixes",
			[]string{
				"mariadb/physicalbackup-20231218155800.xb",
				"mariadb/physicalbackup-20231218160700.xb",
			},
			mustParseMariadbDate("20231218160700"),
			"mariadb/physicalbackup-20231218160700.xb",
			false,
		),
	)
})

var _ = Describe("PhysicalGetOldBackupFiles", func() {
	p := NewPhysicalBackupProcessor()
	DescribeTable("returns the old backup files",
		func(nowFn func() time.Time, backupFiles []string, maxRetention time.Duration, wantBackups []string) {
			previousNowFn := now
			DeferCleanup(func() {
				now = previousNowFn
			})
			now = nowFn

			backups := p.GetOldBackupFiles(backupFiles, maxRetention, logger)
			Expect(backups).To(Equal(wantBackups))
		},
		Entry("no backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			nil,
			1*time.Hour,
			nil,
		),
		Entry("invalid backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"physicalbackup.foo.xb",
				"physicalbackup.bar.xb",
				"physicalbackup.xb",
			},
			1*time.Hour,
			nil,
		),
		Entry("no old backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
			24*time.Hour,
			nil,
		),
		Entry("multiple old backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
			8*time.Hour,
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
			},
		),
		Entry("multiple old backups with invalid",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
				"physicalbackup-foo.xb",
				"physicalbackup-bar.xb",
				"physicalbackup-sql",
			},
			8*time.Hour,
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
			},
		),
		Entry("all old backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
			1*time.Hour,
			[]string{
				"physicalbackup-20231222130000.xb",
				"physicalbackup-20231222140000.xb",
				"physicalbackup-20231222150000.xb",
				"physicalbackup-20231222160000.xb",
				"physicalbackup-20231222170000.xb",
				"physicalbackup-20231222180000.xb",
				"physicalbackup-20231222190000.xb",
				"physicalbackup-20231222200000.xb",
			},
		),
		Entry("prefix",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"mariadb/physicalbackup-20231222130000.xb",
				"mariadb/physicalbackup-20231222140000.xb",
				"mariadb/physicalbackup-20231222150000.xb",
				"mariadb/physicalbackup-20231222160000.xb",
				"mariadb/physicalbackup-20231222170000.xb",
				"mariadb/physicalbackup-20231222180000.xb",
				"mariadb/physicalbackup-20231222190000.xb",
				"mariadb/physicalbackup-20231222200000.xb",
			},
			8*time.Hour,
			[]string{
				"mariadb/physicalbackup-20231222130000.xb",
				"mariadb/physicalbackup-20231222140000.xb",
			},
		),
	)
})

var _ = Describe("PhysicalIsValidBackupFile", func() {
	p := NewPhysicalBackupProcessor()
	DescribeTable("returns whether the backup file is valid",
		func(backupFile string, wantValid bool) {
			valid := p.IsValidBackupFile(backupFile)
			Expect(valid).To(Equal(wantValid))
		},
		Entry("empty", "", false),
		Entry("no date", "physicalbackup.xb", false),
		Entry("no prefix", "202312181614.xb", false),
		Entry("no extension", "physicalbackup-20231218161400", false),
		Entry("invalid date", "physicalbackup-202312181614.xb", false),
		Entry("invalid compression", "physicalbackup-202312181614.xb.foo", false),
		Entry("valid", "physicalbackup-20231218161400.xb", true),
		Entry("valid with gzip", "physicalbackup-20231218161400.xb.gz", true),
		Entry("valid with bzip2", "physicalbackup-20231218161400.xb.bz2", true),
		Entry("valid with prefix", "mariadb/physicalbackup-20231218161400.xb", true),
		Entry("valid with prefix and compression", "mariadb/physicalbackup-20231218161400.xb.gz", true),
	)
})

var _ = Describe("PhysicalParseCompressionAlgorithm", func() {
	p := NewPhysicalBackupProcessor()
	DescribeTable("parses the compression algorithm",
		func(fileName string, wantCompress mariadbv1alpha1.CompressAlgorithm, wantErr bool) {
			compress, err := p.ParseCompressionAlgorithm(fileName)
			Expect(compress).To(Equal(wantCompress))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", "", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("invalid", "foo", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("no compression", "physicalbackup-20231222130000.xb", mariadbv1alpha1.CompressNone, false),
		Entry("invalid compression", "physicalbackup-20231222130000.xb.foo", mariadbv1alpha1.CompressAlgorithm(""), true),
		Entry("gzip", "physicalbackup-20231222130000.xb.gz", mariadbv1alpha1.CompressGzip, false),
		Entry("bzip2", "physicalbackup-20231222130000.xb.bz2", mariadbv1alpha1.CompressBzip2, false),
		Entry("gzip and prefix", "mariadb/physicalbackup-20231222130000.xb.gz", mariadbv1alpha1.CompressGzip, false),
		Entry("bzip2 and prefix", "mariadb/physicalbackup-20231222130000.xb.bz2", mariadbv1alpha1.CompressBzip2, false),
	)
})

var _ = Describe("PhysicalGetUncompressedBackupFile", func() {
	p := NewPhysicalBackupProcessor()
	DescribeTable("returns the uncompressed backup file",
		func(fileName, wantFileName string, wantErr bool) {
			fileName, err := p.GetUncompressedBackupFile(fileName)
			Expect(fileName).To(Equal(wantFileName))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", "", "", true),
		Entry("invalid", "foo", "", true),
		Entry("invalid format", "physicalbackup.xb", "", true),
		Entry("no compression", "physicalbackup-20231222130000.xb", "", true),
		Entry("invalid compression", "physicalbackup-20231222130000.xb.foo", "", true),
		Entry("gzip", "physicalbackup-20231222130000.xb.gz", "physicalbackup-20231222130000.xb", false),
		Entry("bzip2", "physicalbackup-20231222130000.xb.bz2", "physicalbackup-20231222130000.xb", false),
		Entry("prefix and gzip", "mariadb/physicalbackup-20231222130000.xb.gz", "mariadb/physicalbackup-20231222130000.xb", false),
		Entry("prefix and bzip2", "mariadb/physicalbackup-20231222130000.xb.bz2", "mariadb/physicalbackup-20231222130000.xb", false),
	)
})

var _ = Describe("SnapshotGetTargetFile", func() {
	p := NewPhysicalBackupProcessor(
		WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)
	DescribeTable("returns the backup target file",
		func(backupFiles []string, targetRecovery time.Time, wantFile string, wantErr bool) {
			file, err := p.GetBackupTargetFile(backupFiles, targetRecovery, logger)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(file).To(Equal(wantFile))
		},
		Entry("no backups",
			[]string{},
			time.Now(),
			"",
			true,
		),
		Entry("invalid backups",
			[]string{
				"snapshot.foo",
				"snapshot.bar",
				"snapshot",
			},
			time.Now(),
			"",
			true,
		),
		Entry("single backup",
			[]string{
				"snapshot-20231218155800",
			},
			time.Now(),
			"snapshot-20231218155800",
			false,
		),
		Entry("multiple backups with invalid",
			[]string{
				"snapshot-20231218155800",
				"snapshot.foo",
				"snapshot.bar",
			},
			mustParseMariadbDate("20231218155900"),
			"snapshot-20231218155800",
			false,
		),
		Entry("fine grained",
			[]string{
				"snapshot-20231218155800",
				"snapshot-20231218155801",
				"snapshot-20231218160000",
			},
			mustParseMariadbDate("20231218155900"),
			"snapshot-20231218155801",
			false,
		),
		Entry("target before backups",
			[]string{
				"snapshot-20231218155800",
				"snapshot-20231218155900",
			},
			time.UnixMilli(0),
			"",
			true,
		),
		Entry("target after backups",
			[]string{
				"snapshot-20231218155800",
				"snapshot-20231218161300",
			},
			time.Now(),
			"snapshot-20231218161300",
			false,
		),
		Entry("exact target",
			[]string{
				"snapshot-20231218155800",
				"snapshot-20231218160700",
			},
			mustParseMariadbDate("20231218160700"),
			"snapshot-20231218160700",
			false,
		),
	)
})

var _ = Describe("SnapshotGetOldBackupFiles", func() {
	p := NewPhysicalBackupProcessor(
		WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)
	DescribeTable("returns the old backup files",
		func(nowFn func() time.Time, backupFiles []string, maxRetention time.Duration, wantBackups []string) {
			previousNowFn := now
			DeferCleanup(func() {
				now = previousNowFn
			})
			now = nowFn

			backups := p.GetOldBackupFiles(backupFiles, maxRetention, logger)
			Expect(backups).To(Equal(wantBackups))
		},
		Entry("no backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			nil,
			1*time.Hour,
			nil,
		),
		Entry("invalid backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"snapshot.foo",
				"snapshot.bar",
				"snapshot",
			},
			1*time.Hour,
			nil,
		),
		Entry("no old backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
			24*time.Hour,
			nil,
		),
		Entry("multiple old backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
			8*time.Hour,
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
			},
		),
		Entry("multiple old backups with invalid",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
				"snapshot-foo",
				"snapshot-bar",
				"snapshot-sql",
			},
			8*time.Hour,
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
			},
		),
		Entry("all old backups",
			testTimeFn(mustParseMariadbDate("20231222221000")),
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
			1*time.Hour,
			[]string{
				"snapshot-20231222130000",
				"snapshot-20231222140000",
				"snapshot-20231222150000",
				"snapshot-20231222160000",
				"snapshot-20231222170000",
				"snapshot-20231222180000",
				"snapshot-20231222190000",
				"snapshot-20231222200000",
			},
		),
	)
})

var _ = Describe("SnapshotIsValidBackupFile", func() {
	p := NewPhysicalBackupProcessor(
		WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)
	DescribeTable("returns whether the backup file is valid",
		func(backupFile string, wantValid bool) {
			valid := p.IsValidBackupFile(backupFile)
			Expect(valid).To(Equal(wantValid))
		},
		Entry("empty", "", false),
		Entry("no date", "snapshot", false),
		Entry("no prefix", "202312181614", false),
		Entry("invalid date", "snapshot-202312181614", false),
		Entry("valid", "snapshot-20231218161400", true),
	)
})

func testTimeFn(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func mustParseDate(dateString string) time.Time {
	target, err := time.Parse(timeLayout, dateString)
	Expect(err).NotTo(HaveOccurred())
	return target
}

func mustParseMariadbDate(dateString string) time.Time {
	target, err := mdbtime.Parse(dateString)
	Expect(err).NotTo(HaveOccurred())
	return target
}
