package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComposeGtids(t *testing.T) {
	tests := []struct {
		name        string
		rawGtid     string
		rawExternal string
		expected    string
		wantErr     bool
	}{
		{
			name:        "single domain each",
			rawGtid:     "2-211-5",
			rawExternal: "1-10-9",
			expected:    "1-10-9,2-211-5",
		},
		{
			name:        "external multi-domain overrides shared domain",
			rawGtid:     "2-211-5",
			rawExternal: "1-10-9,2-211-30",
			expected:    "1-10-9,2-211-30",
		},
		{
			name:        "local multi-domain, external single overrides shared domain",
			rawGtid:     "1-10-2,2-211-5",
			rawExternal: "1-10-9",
			expected:    "1-10-9,2-211-5",
		},
		{
			name:        "empty local adopts external",
			rawGtid:     "",
			rawExternal: "1-10-9,2-211-30",
			expected:    "1-10-9,2-211-30",
		},
		{
			name:        "empty external keeps local",
			rawGtid:     "1-10-2,2-211-5",
			rawExternal: "",
			expected:    "1-10-2,2-211-5",
		},
		{
			name:        "both empty",
			rawGtid:     "",
			rawExternal: "",
			expected:    "",
		},
		{
			name:        "invalid external GTID",
			rawGtid:     "1-10-2",
			rawExternal: "1-10-216,2-211-18359-extra",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := composeGtids(tt.rawGtid, tt.rawExternal)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
