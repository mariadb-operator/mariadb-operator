package replication

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestGtidsEqualInDomain(t *testing.T) {
	tests := []struct {
		name      string
		gtid      string
		otherGtid string
		domainId  uint32
		want      bool
	}{
		{
			name:      "same position",
			gtid:      "0-10-421",
			otherGtid: "0-10-421",
			want:      true,
		},
		{
			name:      "replica behind",
			gtid:      "0-10-419",
			otherGtid: "0-10-421",
			want:      false,
		},
		{
			name:      "diverged replica",
			gtid:      "0-10-422",
			otherGtid: "0-10-421",
			want:      false,
		},
		{
			name:      "other server with the same sequence number",
			gtid:      "0-11-421",
			otherGtid: "0-10-421",
			want:      false,
		},
		{
			name:      "same position in the domain, other domains differ",
			gtid:      "0-10-421,1-10-7",
			otherGtid: "1-10-9,0-10-421",
			want:      true,
		},
		{
			name:      "same position in another domain",
			gtid:      "0-10-421,1-10-7",
			otherGtid: "0-10-421,1-10-7",
			domainId:  1,
			want:      true,
		},
		{
			name:      "empty position",
			gtid:      "",
			otherGtid: "0-10-421",
			want:      false,
		},
		{
			name:      "invalid position",
			gtid:      "foo",
			otherGtid: "0-10-421",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, gtidsEqualInDomain(tt.gtid, tt.otherGtid, tt.domainId, logr.Discard()))
		})
	}
}
