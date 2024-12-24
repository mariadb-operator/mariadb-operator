package certificate

import (
	"testing"
	"time"
)

func TestRenewalTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                  string
		notBefore             time.Time
		notAfter              time.Time
		renewBeforePercentage int32
		expectedRenewalTime   time.Time
		expectError           bool
	}{
		{
			name:                  "invalid percentage zero",
			notBefore:             now,
			notAfter:              now.Add(24 * time.Hour),
			renewBeforePercentage: 0,
			expectedRenewalTime:   time.Time{},
			expectError:           true,
		},
		{
			name:                  "invalid percentage 100",
			notBefore:             now,
			notAfter:              now.Add(24 * time.Hour),
			renewBeforePercentage: 100,
			expectedRenewalTime:   time.Time{},
			expectError:           true,
		},
		{
			name:                  "50% of 1 day",
			notBefore:             now,
			notAfter:              now.Add(24 * time.Hour),
			renewBeforePercentage: 50,
			expectedRenewalTime:   now.Add(12 * time.Hour),
			expectError:           false,
		},
		{
			name:                  "30% of 3 months",
			notBefore:             now,
			notAfter:              now.Add(3 * 730 * time.Hour), // 3 months
			renewBeforePercentage: 30,
			expectedRenewalTime:   now.Add(3 * 511 * time.Hour), // 70% of 3 months
			expectError:           false,
		},
		{
			name:                  "30% of 3 years",
			notBefore:             now,
			notAfter:              now.Add(3 * 12 * 730 * time.Hour), // 3 years
			renewBeforePercentage: 30,
			expectedRenewalTime:   now.Add(3 * 12 * 511 * time.Hour), // 70% of 3 years
			expectError:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renewalTime, err := getRenewalTime(tt.notBefore, tt.notAfter, tt.renewBeforePercentage)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
			if !tt.expectError && !renewalTime.Truncate(time.Second).Equal(tt.expectedRenewalTime.Truncate(time.Second)) {
				t.Errorf("expected renewal time: %v, got: %v", tt.expectedRenewalTime, renewalTime)
			}
		})
	}
}
