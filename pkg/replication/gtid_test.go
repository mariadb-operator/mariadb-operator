package replication

import (
	"encoding/json"
	"testing"
)

func TestParseGtid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantGtid *Gtid
		wantErr  bool
	}{
		{
			name:     "empty",
			input:    "",
			wantGtid: nil,
			wantErr:  true,
		},
		{
			name:  "all zero",
			input: "0-0-0",
			wantGtid: &Gtid{
				DomainID:   0,
				ServerID:   0,
				SequenceID: 0,
			},
			wantErr: false,
		},
		{
			name:  "normal values",
			input: "1-2-3",
			wantGtid: &Gtid{
				DomainID:   1,
				ServerID:   2,
				SequenceID: 3,
			},
			wantErr: false,
		},
		{
			name:  "max values",
			input: "4294967295-4294967295-18446744073709551615",
			wantGtid: &Gtid{
				DomainID:   4294967295,
				ServerID:   4294967295,
				SequenceID: 18446744073709551615,
			},
			wantErr: false,
		},
		{
			name:     "contains comma (multi-source)",
			input:    "1,2-3-4",
			wantGtid: nil,
			wantErr:  true,
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseGtid(tc.input)
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

func TestGtidMarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		gtid       Gtid
		wantString string
		wantErr    bool
	}{
		{
			name:       "empty",
			gtid:       Gtid{},
			wantString: `"0-0-0"`,
			wantErr:    false,
		},
		{
			name:       "all zero",
			gtid:       Gtid{DomainID: 0, ServerID: 0, SequenceID: 0},
			wantString: `"0-0-0"`,
			wantErr:    false,
		},
		{
			name:       "normal",
			gtid:       Gtid{DomainID: 1, ServerID: 2, SequenceID: 3},
			wantString: `"1-2-3"`,
			wantErr:    false,
		},
		{
			name: "max values",
			gtid: Gtid{
				DomainID:   4294967295,
				ServerID:   4294967295,
				SequenceID: 18446744073709551615,
			},
			wantString: `"4294967295-4294967295-18446744073709551615"`,
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bytes, err := tc.gtid.MarshalJSON()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil and bytes: %s", string(bytes))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(bytes) != tc.wantString {
				t.Fatalf("marshal mismatch: got %s, want %s", string(bytes), tc.wantString)
			}
			// custom marshaller
			customBytes, err := json.Marshal(&tc.gtid)
			if err != nil {
				t.Fatalf("json.Marshal unexpected error: %v", err)
			}
			if string(customBytes) != tc.wantString {
				t.Fatalf("json.Marshal mismatch: got %s, want %s", string(customBytes), tc.wantString)
			}
		})
	}
}

func TestGtidUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantGtid *Gtid
		wantErr  bool
	}{
		{
			name:     "empty",
			input:    []byte{},
			wantGtid: &Gtid{},
			wantErr:  true,
		},
		{
			name:     "null",
			input:    []byte("null"),
			wantGtid: &Gtid{},
		},
		{
			name:  "valid string",
			input: []byte(`"1-2-3"`),
			wantGtid: &Gtid{
				DomainID:   1,
				ServerID:   2,
				SequenceID: 3,
			},
		},
		{
			name:    "invalid json type (number)",
			input:   []byte("123"),
			wantErr: true,
		},
		{
			name:    "invalid gtid format (too few parts)",
			input:   []byte(`"1-2"`),
			wantErr: true,
		},
		{
			name:    "contains comma (multi-source)",
			input:   []byte(`"1,2-3-4"`),
			wantErr: true,
		},
		{
			name:    "non-numeric domain",
			input:   []byte(`"a-2-3"`),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gtid Gtid
			err := (&gtid).UnmarshalJSON(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %s, got nil and result %+v", string(tc.input), gtid)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %s: %v", string(tc.input), err)
			}
			if tc.wantGtid == nil {
				// nothing to check
				return
			}
			if gtid.DomainID != tc.wantGtid.DomainID || gtid.ServerID != tc.wantGtid.ServerID || gtid.SequenceID != tc.wantGtid.SequenceID {
				t.Fatalf("unmarshal mismatch for input %s: got %+v, want %+v", string(tc.input), gtid, tc.wantGtid)
			}
		})
	}
}
