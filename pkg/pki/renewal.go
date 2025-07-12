package pki

import (
	"fmt"
	"time"
)

// DefaultRenewBeforePercentage is the default percentage to calculate the renewal duration.
var DefaultRenewBeforePercentage = int32(33) // 33%

// RenewalDuration calculates the certificate renewal duration based on a given duration and a specified percentage.
// The percentage determines the fraction of the duration before the expiration when renewal should occur.
func RenewalDuration(duration time.Duration, renewBeforePercentage int32) (*time.Duration, error) {
	if err := validateRenewBeforePercentage(renewBeforePercentage); err != nil {
		return nil, err
	}
	// See https://github.com/cert-manager/cert-manager/blob/dd8b7d233110cbd49f2f31eb709f39865f8b0300/pkg/util/pki/renewaltime.go#L71
	renewalDuration := duration * time.Duration(renewBeforePercentage) / 100

	return &renewalDuration, nil
}

// RenewalTime calculates the renewal time for a fraction based on its lifetime.
// The percentage determines the fraction of the validity period before expiration when renewal should occur.
func RenewalTime(notBefore, notAfter time.Time, renewBeforePercentage int32) (*time.Time, error) {
	if err := validateRenewBeforePercentage(renewBeforePercentage); err != nil {
		return nil, err
	}
	duration := notAfter.Sub(notBefore)
	renewalDuration, err := RenewalDuration(duration, renewBeforePercentage)
	if err != nil {
		return nil, fmt.Errorf("error getting renewal duration: %v", err)
	}
	// See https://github.com/cert-manager/cert-manager/blob/dd8b7d233110cbd49f2f31eb709f39865f8b0300/pkg/util/pki/renewaltime.go#L53
	renewalTime := notAfter.Add(-1 * *renewalDuration).Truncate(time.Second)

	return &renewalTime, nil
}

func validateRenewBeforePercentage(renewBeforePercentage int32) error {
	if renewBeforePercentage < 10 || renewBeforePercentage > 90 {
		return fmt.Errorf("invalid renewBeforePercentage %v, it must be between [10, 90]", renewBeforePercentage)
	}
	return nil
}
