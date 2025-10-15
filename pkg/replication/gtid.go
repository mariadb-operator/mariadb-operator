package replication

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Gtid is a Global Transaction ID. See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#implementation.
// +kubebuilder:validation:Type=string
type Gtid struct {
	DomainID   uint32 `json:"-"` // ignored by default JSON gen
	ServerID   uint32 `json:"-"`
	SequenceID uint64 `json:"-"`
}

func (g *Gtid) String() string {
	return fmt.Sprintf("%d-%d-%d", g.DomainID, g.ServerID, g.SequenceID)
}

func (g *Gtid) MarshalJSON() ([]byte, error) {
	return json.Marshal(g.String())
}

func (g *Gtid) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*g = Gtid{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("GTID must be a JSON string: %w", err)
	}
	parsed, err := ParseGtid(s)
	if err != nil {
		return err
	}
	*g = *parsed
	return nil
}

func (g *Gtid) Equal(o *Gtid) bool {
	if g == nil || o == nil {
		return false
	}
	return g.DomainID == o.DomainID &&
		g.ServerID == o.ServerID &&
		g.SequenceID == o.SequenceID
}

func (g *Gtid) GreaterThan(o *Gtid) (bool, error) {
	if g == nil || o == nil {
		return false, nil
	}
	if g.DomainID != o.DomainID {
		return false, fmt.Errorf("domain IDs are different (%d and %d). Not comparable", g.DomainID, o.DomainID)
	}
	return g.SequenceID > o.SequenceID, nil
}

func ParseGtid(rawGtid string) (*Gtid, error) {
	if strings.Contains(rawGtid, ",") {
		return nil, fmt.Errorf("multi-source replication not supported. Detected multiple GTID values in: %s", rawGtid)
	}
	parts := strings.Split(rawGtid, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid GTID format: expected 3 parts (domain-server-sequence), got %d", len(parts))
	}

	domainID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid domain ID: %v", err)
	}
	serverID, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %v", err)
	}
	sequenceID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid sequence ID: %v", err)
	}

	return &Gtid{
		DomainID:   uint32(domainID),
		ServerID:   uint32(serverID),
		SequenceID: sequenceID,
	}, nil
}
