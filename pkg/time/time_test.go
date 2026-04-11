package time_test

import (
	gotime "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/time"
)

var _ = Describe("Format", func() {
	DescribeTable("formats time to string",
		func(input gotime.Time, want string) {
			Expect(time.Format(input)).To(Equal(want))
		},
		Entry("UTC time", gotime.Date(2024, 6, 1, 15, 4, 5, 0, gotime.UTC), "20240601150405"),
		Entry("Different year and month", gotime.Date(1999, 12, 31, 23, 59, 59, 0, gotime.UTC), "19991231235959"),
		Entry("Non-UTC time zone", gotime.Date(2024, 6, 1, 15, 4, 5, 0, gotime.FixedZone("CET", 2*60*60)), "20240601130405"),
	)
})

var _ = Describe("Parse", func() {
	DescribeTable("parses string to time",
		func(input string, want gotime.Time, wantErr bool) {
			result, err := time.Parse(input)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Equal(want)).To(BeTrue())
				Expect(result.Location()).To(Equal(gotime.UTC))
			}
		},
		Entry("Valid timestamp", "20240601150405", gotime.Date(2024, 6, 1, 15, 4, 5, 0, gotime.UTC), false),
		Entry("Another valid timestamp", "19991231235959", gotime.Date(1999, 12, 31, 23, 59, 59, 0, gotime.UTC), false),
		Entry("Invalid format - too short", "20240601", gotime.Time{}, true),
		Entry("Invalid format - non-numeric", "2024ABCD150405", gotime.Time{}, true),
		Entry("Invalid date - impossible month", "20241301150405", gotime.Time{}, true),
		Entry("Invalid date - impossible day", "20240632150405", gotime.Time{}, true),
		Entry("Empty string", "", gotime.Time{}, true),
	)
})
