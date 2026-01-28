package replication

import (
	"testing"

	"github.com/go-logr/logr"
)

func TestParseGtid(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name         string
		input        string
		gtidDomainId uint32
		wantGtid     *Gtid
		wantErr      bool
	}{
		{
			name:         "empty",
			input:        "",
			gtidDomainId: 0,
			wantGtid:     nil,
			wantErr:      true,
		},
		{
			name:         "invalid",
			input:        "foo",
			gtidDomainId: 0,
			wantGtid:     nil,
			wantErr:      true,
		},
		{
			name:     "too few parts",
			input:    "1-2",
			wantGtid: nil,
			wantErr:  true,
		},
		{
			name:     "too many parts",
			input:    "1-2-3-4",
			wantGtid: nil,
			wantErr:  true,
		},
		{
			name:     "non-numeric domain",
			input:    "a-2-3",
			wantGtid: nil,
			wantErr:  true,
		},
		{
			name:     "non-numeric server",
			input:    "1-b-3",
			wantGtid: nil,
			wantErr:  true,
		},
		{
			name:     "non-numeric sequence",
			input:    "1-2-c",
			wantGtid: nil,
			wantErr:  true,
		},
		{
			name:         "all zero",
			input:        "0-0-0",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   0,
				SequenceID: 0,
			},
			wantErr: false,
		},
		{
			name:         "valid",
			input:        "0-2001-48431",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2001,
				SequenceID: 48431,
			},
			wantErr: false,
		},
		{
			name:         "max values",
			input:        "0-4294967295-18446744073709551615",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   4294967295,
				SequenceID: 18446744073709551615,
			},
			wantErr: false,
		},
		{
			name:         "multiple GTID, some invalid",
			input:        "2-a-48438,0-2001-48431,1-2101-48436",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2001,
				SequenceID: 48431,
			},
			wantErr: false,
		},
		{
			name:         "multiple GTID, some empty",
			input:        ",0-2002-48432",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2002,
				SequenceID: 48432,
			},
			wantErr: false,
		},
		{
			name:         "multiple GTID from same domain",
			input:        "0-2001-48431,0-2002-48432",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2001,
				SequenceID: 48431,
			},
			wantErr: false,
		},
		{
			name:         "1. multiple GTID from different domains",
			input:        "2-2201-48438,1-2101-48436,0-2001-48431",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2001,
				SequenceID: 48431,
			},
			wantErr: false,
		},
		{
			name:         "2. multiple GTID from different domains",
			input:        "0-2001-48431,2-2201-48438,1-2101-48436",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2001,
				SequenceID: 48431,
			},
			wantErr: false,
		},
		{
			name:         "3. multiple GTID from different domains",
			input:        "2-2201-48438,0-2001-48431,1-2101-48436",
			gtidDomainId: 0,
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   2001,
				SequenceID: 48431,
			},
			wantErr: false,
		},
		{
			name:         "multiple GTID from different domains using non default domain",
			input:        "2-2201-48438,1-2101-48436,0-2001-48431",
			gtidDomainId: 1,
			wantGtid: &Gtid{
				DomainID:   1,
				ServerID:   2101,
				SequenceID: 48436,
			},
			wantErr: false,
		},
		{
			name:         "domain not found",
			input:        "2-2201-48438,1-2101-48436,0-2001-48431",
			gtidDomainId: 5,
			wantGtid:     nil,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseGtidWithDomainId(tc.input, tc.gtidDomainId, logger)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil and result %#v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got == nil {
				t.Fatalf("expected non-nil result for input %q", tc.input)
			}
			if got.DomainID != tc.wantGtid.DomainID || got.ServerID != tc.wantGtid.ServerID || got.SequenceID != tc.wantGtid.SequenceID {
				t.Fatalf("parse mismatch for input %q: got %+v, want %+v", tc.input, got, tc.wantGtid)
			}
		})
	}
}
