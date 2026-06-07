package replication

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseGtidWithDomainId", func() {
	logger := logr.Discard()

	DescribeTable("parses GTID with domain id",
		func(input string, gtidDomainId uint32, wantGtid *Gtid, wantErr bool) {
			got, err := ParseGtidWithDomainId(input, gtidDomainId, logger)
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(got).NotTo(BeNil())
			Expect(got.DomainID).To(Equal(wantGtid.DomainID))
			Expect(got.ServerID).To(Equal(wantGtid.ServerID))
			Expect(got.SequenceID).To(Equal(wantGtid.SequenceID))
		},
		Entry("empty", "", uint32(0), (*Gtid)(nil), true),
		Entry("invalid", "foo", uint32(0), (*Gtid)(nil), true),
		Entry("too few parts", "1-2", uint32(0), (*Gtid)(nil), true),
		Entry("too many parts", "1-2-3-4", uint32(0), (*Gtid)(nil), true),
		Entry("non-numeric domain", "a-2-3", uint32(0), (*Gtid)(nil), true),
		Entry("non-numeric server", "1-b-3", uint32(0), (*Gtid)(nil), true),
		Entry("non-numeric sequence", "1-2-c", uint32(0), (*Gtid)(nil), true),
		Entry("all zero", "0-0-0", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   0,
			SequenceID: 0,
		}, false),
		Entry("valid", "0-2001-48431", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2001,
			SequenceID: 48431,
		}, false),
		Entry("max values", "0-4294967295-18446744073709551615", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   4294967295,
			SequenceID: 18446744073709551615,
		}, false),
		Entry("multiple GTID, some invalid", "2-a-48438,0-2001-48431,1-2101-48436", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2001,
			SequenceID: 48431,
		}, false),
		Entry("multiple GTID, some empty", ",0-2002-48432", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2002,
			SequenceID: 48432,
		}, false),
		Entry("multiple GTID from same domain", "0-2001-48431,0-2002-48432", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2001,
			SequenceID: 48431,
		}, false),
		Entry("1. multiple GTID from different domains", "2-2201-48438,1-2101-48436,0-2001-48431", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2001,
			SequenceID: 48431,
		}, false),
		Entry("2. multiple GTID from different domains", "0-2001-48431,2-2201-48438,1-2101-48436", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2001,
			SequenceID: 48431,
		}, false),
		Entry("3. multiple GTID from different domains", "2-2201-48438,0-2001-48431,1-2101-48436", uint32(0), &Gtid{
			DomainID:   0,
			ServerID:   2001,
			SequenceID: 48431,
		}, false),
		Entry("multiple GTID from different domains using non default domain", "2-2201-48438,1-2101-48436,0-2001-48431", uint32(1), &Gtid{
			DomainID:   1,
			ServerID:   2101,
			SequenceID: 48436,
		}, false),
		Entry("domain not found", "2-2201-48438,1-2101-48436,0-2001-48431", uint32(5), (*Gtid)(nil), true),
	)
})

var _ = Describe("ParseRawGtidInMetaFile", func() {
	DescribeTable("parses raw GTID in meta file",
		func(input string, wantGtid string, wantErr bool) {
			got, err := ParseRawGtidInMetaFile([]byte(input))
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(got).To(Equal(wantGtid))
		},
		Entry("empty file", "", "", true),
		Entry("one field only", "mariadb-repl-bin.000003", "", true),
		Entry("two fields only", "mariadb-repl-bin.000004 456", "", true),
		Entry("valid format", "mariadb-repl-bin.000001 335 0-10-9", "0-10-9", false),
		Entry("extra spaces and newline", "  mariadb-repl-bin.000002   123    1-2-3  \n", "1-2-3", false),
		Entry("tabs between fields", "bin\t12\t2-3-4", "2-3-4", false),
	)
})
