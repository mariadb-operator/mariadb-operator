package client

import (
	"encoding/json"
	"testing"
)

func TestRelationshipsMarshal(t *testing.T) {
	tests := []struct {
		name string
		rels *Relationships
		want string
	}{
		{
			name: "empty",
			rels: NewRelationshipsBuilder().Build(),
			want: `{}`,
		},
		{
			name: "servers",
			rels: NewRelationshipsBuilder().WithServers("mariadb-0", "mariadb-1").Build(),
			want: `{"servers":{"data":[{"id":"mariadb-0","type":"servers"},{"id":"mariadb-1","type":"servers"}]}}`,
		},
		{
			// the other relationship types must keep omitting an empty data member, otherwise MaxScale
			// detaches every object instead of leaving the relationship untouched
			name: "no servers",
			rels: NewRelationshipsBuilder().WithServers().Build(),
			want: `{"servers":{}}`,
		},
		{
			name: "no monitors",
			rels: NewRelationshipsBuilder().WithMonitors().Build(),
			want: `{"monitors":{}}`,
		},
		{
			name: "no services",
			rels: NewRelationshipsBuilder().WithServices().Build(),
			want: `{"services":{}}`,
		},
		{
			name: "no listeners",
			rels: NewRelationshipsBuilder().WithListeners().Build(),
			want: `{"listeners":{}}`,
		},
		{
			name: "filters",
			rels: NewRelationshipsBuilder().WithFilters("throttle", "qla").Build(),
			want: `{"filters":{"data":[{"id":"throttle","type":"filters"},{"id":"qla","type":"filters"}]}}`,
		},
		{
			// unlike the rest, an empty filters relationship must be marshaled so that removing every
			// filter from a service detaches them in MaxScale
			name: "no filters",
			rels: NewRelationshipsBuilder().WithFilters().Build(),
			want: `{"filters":{"data":[]}}`,
		},
		{
			name: "servers and filters",
			rels: NewRelationshipsBuilder().WithServers("mariadb-0").WithFilters().Build(),
			want: `{"servers":{"data":[{"id":"mariadb-0","type":"servers"}]},"filters":{"data":[]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(tt.rels)
			if err != nil {
				t.Fatalf("unexpected error marshaling relationships: %v", err)
			}
			if got := string(bytes); got != tt.want {
				t.Errorf("unexpected relationships, got: %s, want: %s", got, tt.want)
			}
		})
	}
}

func TestNewFilterRelationshipData(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		want    string
	}{
		{
			name:    "nil filters",
			filters: nil,
			want:    `{"data":[]}`,
		},
		{
			name:    "no filters",
			filters: []string{},
			want:    `{"data":[]}`,
		},
		{
			name:    "single filter",
			filters: []string{"throttle"},
			want:    `{"data":[{"id":"throttle","type":"filters"}]}`,
		},
		{
			// filters are applied in the order they are declared in the service
			name:    "multiple filters keep order",
			filters: []string{"qla", "throttle"},
			want:    `{"data":[{"id":"qla","type":"filters"},{"id":"throttle","type":"filters"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := json.Marshal(NewFilterRelationshipData(tt.filters...))
			if err != nil {
				t.Fatalf("unexpected error marshaling filter relationship data: %v", err)
			}
			if got := string(bytes); got != tt.want {
				t.Errorf("unexpected filter relationship data, got: %s, want: %s", got, tt.want)
			}
		})
	}
}
