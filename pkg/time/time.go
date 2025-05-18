package time

import (
	"fmt"
	"time"
)

// Simplified version of time.RFC3339 compatible with Kubernetes DNS names. It displays the time in UTC.
const timeLayout = "20060102150405"

// Format returns the UTC time formatted in a compact, DNS-safe format (YYYYMMDDHHMMSS).
func Format(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

// Parse parses a compact timestamp (YYYYMMDDHHMMSS) and returns the UTC time.
func Parse(tRaw string) (time.Time, error) {
	t, err := time.Parse(timeLayout, tRaw)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing time \"%s\": %v", tRaw, err)
	}
	return t.UTC(), nil
}
