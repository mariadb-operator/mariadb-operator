package binlog

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	mariadbrepl "github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

var _ = Describe("BuildTimeline", func() {
	parseTestFile := func(file string) *BinlogIndex {
		testFile := filepath.Join("test", file)
		bytes, err := os.ReadFile(testFile)
		Expect(err).NotTo(HaveOccurred())
		var bi BinlogIndex
		Expect(yaml.Unmarshal(bytes, &bi)).To(Succeed())
		return &bi
	}
	parseGtid := func(s string) *mariadbrepl.Gtid {
		g, err := mariadbrepl.ParseGtid(s)
		Expect(err).NotTo(HaveOccurred())
		return g
	}

	DescribeTable("building a timeline",
		func(file string, gtidStr string, targetTime time.Time, strictMode bool, wantPath []string, wantErr bool) {
			indexFile := parseTestFile(file)
			startGtid := parseGtid(gtidStr)

			binlogMetas, err := indexFile.BuildTimeline(startGtid, targetTime, strictMode, logr.Discard())
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(getBinlogTimeline(binlogMetas)).To(Equal(wantPath))
			}
		},
		Entry("single binlog",
			"single-binlog.yaml", "0-10-1", time.Now(), false,
			[]string{
				"server-10/mariadb-repl-bin.000002",
			},
			false,
		),
		Entry("single binlog - strict",
			"single-binlog.yaml", "0-10-1", time.Now(), true,
			nil,
			true,
		),
		Entry("multiple binlogs",
			"multiple-binlogs.yaml", "0-10-1", mustParseDate("2026-01-20T11:11:26Z"), false,
			[]string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
				"server-10/mariadb-repl-bin.000007",
				"server-10/mariadb-repl-bin.000008",
				"server-10/mariadb-repl-bin.000009",
				"server-10/mariadb-repl-bin.000010",
				"server-10/mariadb-repl-bin.000011",
			},
			false,
		),
		Entry("multiple binlogs - strict",
			"multiple-binlogs.yaml", "0-10-1", time.Now(), true,
			nil,
			true,
		),
		Entry("filter by server-10 gtid and date",
			"failover-1205-1208.yaml", "0-10-40", mustParseDate("2026-02-04T12:05:00Z"), true,
			[]string{
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
			},
			false,
		),
		Entry("filter by server-11 gtid and date",
			"failover-1205-1208.yaml", "0-11-100", mustParseDate("2026-02-04T12:06:56Z"), true,
			[]string{
				"server-11/mariadb-repl-bin.000002",
				"server-11/mariadb-repl-bin.000003",
				"server-11/mariadb-repl-bin.000004",
				"server-11/mariadb-repl-bin.000005",
				"server-11/mariadb-repl-bin.000006",
			},
			false,
		),
		Entry("failover",
			"failover.yaml", "0-10-1", time.Now(), false,
			[]string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
			},
			false,
		),
		Entry("failover - strict",
			"failover.yaml", "0-10-1", time.Now(), true,
			nil,
			true,
		),
		Entry("failover no stop event",
			"failover-no-stop-event.yaml", "0-10-1", time.Now(), false,
			[]string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
			},
			false,
		),
		Entry("failover no stop event - strict",
			"failover-no-stop-event.yaml", "0-10-1", time.Now(), true,
			nil,
			true,
		),
		Entry("failover at 12:05",
			"failover-1205-1208.yaml", "0-10-1", mustParseDate("2026-02-04T12:06:39Z"), false,
			[]string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
				// FAILOVER to server-11
				"server-11/mariadb-repl-bin.000001",
				"server-11/mariadb-repl-bin.000002",
				"server-11/mariadb-repl-bin.000003",
				"server-11/mariadb-repl-bin.000004",
				"server-11/mariadb-repl-bin.000005",
			},
			false,
		),
		Entry("failover at 12:05 - strict",
			"failover-1205-1208.yaml", "0-10-1", mustParseDate("2026-02-04T12:06:39Z"), true,
			[]string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
				// FAILOVER to server-11
				"server-11/mariadb-repl-bin.000001",
				"server-11/mariadb-repl-bin.000002",
				"server-11/mariadb-repl-bin.000003",
				"server-11/mariadb-repl-bin.000004",
				"server-11/mariadb-repl-bin.000005",
				// FAILOVER to server-10
			},
			false,
		),
		Entry("failover at 12:05 and 12:08",
			"failover-1205-1208.yaml", "0-10-1", mustParseDate("2026-02-04T12:08:32Z"), false,
			[]string{
				"server-10/mariadb-repl-bin.000002",
				"server-10/mariadb-repl-bin.000003",
				"server-10/mariadb-repl-bin.000004",
				"server-10/mariadb-repl-bin.000005",
				"server-10/mariadb-repl-bin.000006",
				// FAILOVER to server-11
				"server-11/mariadb-repl-bin.000001",
				"server-11/mariadb-repl-bin.000002",
				"server-11/mariadb-repl-bin.000003",
				"server-11/mariadb-repl-bin.000004",
				"server-11/mariadb-repl-bin.000005",
				"server-11/mariadb-repl-bin.000006",
				"server-11/mariadb-repl-bin.000007",
				"server-11/mariadb-repl-bin.000008",
				"server-11/mariadb-repl-bin.000009",
				// FAILOVER to server-10
			},
			false,
		),
		Entry("failover at 12:05 and 12:08 - strict",
			"failover-1205-1208.yaml", "0-10-1", mustParseDate("2026-02-04T12:08:32Z"), true,
			nil,
			true,
		),
	)
})

var _ = Describe("ErrNoBinlogs", func() {
	It("should return ErrNoBinlogs when there are no binlogs", func() {
		binlogIndex := &BinlogIndex{
			APIVersion: BinlogIndexV1,
			Binlogs:    make(map[string][]BinlogMetadata),
		}
		startGtid := &mariadbrepl.Gtid{
			DomainID:   1,
			ServerID:   1,
			SequenceID: 1,
		}
		targetTime := time.Now()

		result, err := binlogIndex.BuildTimeline(startGtid, targetTime, false, logr.Discard())

		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrNoBinlogs)).To(BeTrue())
		Expect(result).To(BeNil())
	})
})

func mustParseDate(s string) time.Time {
	d, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return d
}

func getBinlogTimeline(binlogMetas []BinlogMetadata) []string {
	path := make([]string, len(binlogMetas))
	for i, binlogMeta := range binlogMetas {
		path[i] = binlogMeta.ObjectStoragePath()
	}
	return path
}
