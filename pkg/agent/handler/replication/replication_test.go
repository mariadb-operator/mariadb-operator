package replication

import (
	"testing"
)

func TestParseGtid_Table(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantGtid string
		wantErr  bool
	}{
		{
			name:    "empty file",
			input:   "",
			wantErr: true,
		},
		{
			name:    "one field only",
			input:   "mariadb-repl-bin.000003",
			wantErr: true,
		},
		{
			name:    "two fields only",
			input:   "mariadb-repl-bin.000004 456",
			wantErr: true,
		},
		{
			name:     "valid format",
			input:    "mariadb-repl-bin.000001 335 0-10-9",
			wantGtid: "0-10-9",
			wantErr:  false,
		},
		{
			name:     "extra spaces and newline",
			input:    "  mariadb-repl-bin.000002   123    1-2-3  \n",
			wantGtid: "1-2-3",
			wantErr:  false,
		},
		{
			name:     "tabs between fields",
			input:    "bin\t12\t2-3-4",
			wantGtid: "2-3-4",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGtid([]byte(tc.input))
			if tc.wantErr && err == nil {
				t.Fatal("error expected, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantGtid {
				t.Fatalf("gtid mismatch: want=%q got=%q", tc.wantGtid, got)
			}
		})
	}
}
