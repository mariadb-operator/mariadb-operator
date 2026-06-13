package replication

import (
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
)

func TestHasRelayLogEvents(t *testing.T) {
	tests := []struct {
		name       string
		gtidIO     string
		gtidSQL    string
		wantEvents bool
	}{
		{
			name:       "matching single GTID",
			gtidIO:     "0-10-100",
			gtidSQL:    "0-10-100",
			wantEvents: false,
		},
		{
			name:       "IO ahead of SQL",
			gtidIO:     "0-10-101",
			gtidSQL:    "0-10-100",
			wantEvents: true,
		},
		{
			name:       "SQL ahead of IO",
			gtidIO:     "0-10-100",
			gtidSQL:    "0-10-101",
			wantEvents: true,
		},
		{
			name:       "SQL has extra same-domain server",
			gtidIO:     "0-10-30",
			gtidSQL:    "0-10-30,0-11-1747",
			wantEvents: true,
		},
		{
			name:       "same GTID set in different order",
			gtidIO:     "0-11-1747,0-10-30",
			gtidSQL:    "0-10-30,0-11-1747",
			wantEvents: false,
		},
		{
			name:       "different domain ignored",
			gtidIO:     "0-10-30,1-12-200",
			gtidSQL:    "0-10-30,1-12-100",
			wantEvents: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasRelayLogEvents(&mariadbv1alpha1.ReplicaStatusVars{
				GtidIOPos:      &tt.gtidIO,
				GtidCurrentPos: &tt.gtidSQL,
			}, 0, logr.Discard())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantEvents {
				t.Fatalf("expected events=%t, got %t", tt.wantEvents, got)
			}
		})
	}
}
