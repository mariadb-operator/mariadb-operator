package time_test

import (
	"testing"
	gotime "time"

	"github.com/mariadb-operator/mariadb-operator/pkg/time"
	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name  string
		input gotime.Time
		want  string
	}{
		{
			name:  "UTC time",
			input: gotime.Date(2024, 6, 1, 15, 4, 5, 0, gotime.UTC),
			want:  "20240601150405",
		},
		{
			name:  "Different year and month",
			input: gotime.Date(1999, 12, 31, 23, 59, 59, 0, gotime.UTC),
			want:  "19991231235959",
		},
		{
			name:  "Non-UTC time zone",
			input: gotime.Date(2024, 6, 1, 15, 4, 5, 0, gotime.FixedZone("CET", 2*60*60)),
			want:  "20240601130405", // 15:04:05 CET is 13:04:05 UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := time.Format(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    gotime.Time
		wantErr bool
	}{
		{
			name:  "Valid timestamp",
			input: "20240601150405",
			want:  gotime.Date(2024, 6, 1, 15, 4, 5, 0, gotime.UTC),
		},
		{
			name:  "Another valid timestamp",
			input: "19991231235959",
			want:  gotime.Date(1999, 12, 31, 23, 59, 59, 0, gotime.UTC),
		},
		{
			name:    "Invalid format - too short",
			input:   "20240601",
			wantErr: true,
		},
		{
			name:    "Invalid format - non-numeric",
			input:   "2024ABCD150405",
			wantErr: true,
		},
		{
			name:    "Invalid date - impossible month",
			input:   "20241301150405",
			wantErr: true,
		},
		{
			name:    "Invalid date - impossible day",
			input:   "20240632150405",
			wantErr: true,
		},
		{
			name:    "Empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := time.Parse(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.True(t, result.Equal(tt.want), "expected: %v, got: %v", tt.want, result)
				assert.Equal(t, gotime.UTC, result.Location())
			}
		})
	}
}
