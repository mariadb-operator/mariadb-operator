package pki

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenewalDuration", func() {
	DescribeTable("calculates the renewal duration",
		func(duration time.Duration, renewBeforePercentage int32, expectedDuration time.Duration, expectError bool) {
			renewalDuration, err := RenewalDuration(duration, renewBeforePercentage)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(*renewalDuration).To(Equal(expectedDuration))
			}
		},
		Entry("invalid percentage zero", 24*time.Hour, int32(0), time.Duration(0), true),
		Entry("invalid percentage 100", 24*time.Hour, int32(100), time.Duration(0), true),
		Entry("50% of 1 day", 24*time.Hour, int32(50), 12*time.Hour, false),
		Entry("30% of 3 months", 3*730*time.Hour, int32(30), 3*219*time.Hour, false),
		Entry("30% of 3 years", 3*12*730*time.Hour, int32(30), 3*12*219*time.Hour, false),
	)
})

var _ = Describe("RenewalTime", func() {
	now := time.Now()
	DescribeTable("calculates the renewal time",
		func(notBefore, notAfter time.Time, renewBeforePercentage int32, expectedRenewalTime time.Time, expectError bool) {
			renewalTime, err := RenewalTime(notBefore, notAfter, renewBeforePercentage)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(renewalTime.Truncate(time.Second).Equal(expectedRenewalTime.Truncate(time.Second))).To(BeTrue())
			}
		},
		Entry("invalid percentage zero", now, now.Add(24*time.Hour), int32(0), time.Time{}, true),
		Entry("invalid percentage 100", now, now.Add(24*time.Hour), int32(100), time.Time{}, true),
		Entry("50% of 1 day", now, now.Add(24*time.Hour), int32(50), now.Add(12*time.Hour), false),
		Entry("30% of 3 months", now, now.Add(3*730*time.Hour), int32(30), now.Add(3*511*time.Hour), false),
		Entry("30% of 3 years", now, now.Add(3*12*730*time.Hour), int32(30), now.Add(3*12*511*time.Hour), false),
	)
})
