package replication

import (
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
)

// compile-time interface assertions
var (
	_ encoding.TextMarshaler   = (*Gtid)(nil)
	_ encoding.TextUnmarshaler = (*Gtid)(nil)
	_ json.Marshaler           = (*Gtid)(nil)
	_ json.Unmarshaler         = (*Gtid)(nil)
)

// Gtid is a Global Transaction ID. See: https://mariadb.com/docs/server/ha-and-performance/standard-replication/gtid#implementation.
// +kubebuilder:validation:Type=string
type Gtid struct {
	DomainID   uint32
	ServerID   uint32
	SequenceID uint64
}

func (g *Gtid) String() string {
	return fmt.Sprintf("%d-%d-%d", g.DomainID, g.ServerID, g.SequenceID)
}

func (g *Gtid) MarshalText() ([]byte, error) {
	if g == nil {
		return nil, nil
	}
	return []byte(g.String()), nil
}

func (g *Gtid) UnmarshalText(text []byte) error {
	if g == nil {
		return fmt.Errorf("nil Gtid receiver")
	}
	if len(text) == 0 {
		return fmt.Errorf("empty GTID text")
	}
	parsed, err := ParseGtid(string(text))
	if err != nil {
		return err
	}
	*g = *parsed
	return nil
}

func (g *Gtid) MarshalJSON() ([]byte, error) {
	if g == nil {
		return []byte("null"), nil
	}
	return json.Marshal(g.String())
}

func (g *Gtid) UnmarshalJSON(data []byte) error {
	if g == nil {
		return fmt.Errorf("nil Gtid receiver")
	}
	if len(data) == 0 || string(data) == "null" {
		*g = Gtid{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("gtid: expected JSON string or null: %w", err)
	}
	parsed, err := ParseGtid(s)
	if err != nil {
		return err
	}
	*g = *parsed
	return nil
}

func (g *Gtid) LessThan(o *Gtid) (bool, error) {
	if g == nil || o == nil {
		return false, nil
	}
	if g.DomainID != o.DomainID {
		return false, fmt.Errorf("domain IDs are different (%d and %d). Not comparable", g.DomainID, o.DomainID)
	}
	return g.SequenceID < o.SequenceID, nil
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

func (g *Gtid) Diff(o *Gtid) (uint64, error) {
	if g == nil || o == nil {
		return 0, nil
	}
	if g.DomainID != o.DomainID {
		return 0, fmt.Errorf("domain IDs are different (%d and %d). Not comparable", g.DomainID, o.DomainID)
	}
	return uint64(math.Abs(float64(g.SequenceID) - float64(o.SequenceID))), nil
}

func ParseGtidWithDomainId(rawGtid string, domainId uint32, logger logr.Logger) (*Gtid, error) {
	if !strings.Contains(rawGtid, ",") {
		return ParseGtid(rawGtid)
	}
	parts := strings.Split(rawGtid, ",")

	for _, part := range parts {
		rawGtid = strings.TrimSpace(part)
		if part == "" {
			logger.Info("Ignoring empty GTID")
			continue
		}

		gtid, err := ParseGtid(rawGtid)
		if err != nil {
			logger.Error(err, "Error parsing GTID", "gtid", rawGtid)
			continue
		}
		if gtid.DomainID == domainId {
			return gtid, nil
		}
	}
	return nil, fmt.Errorf("GTID for domain ID %d not found", domainId)
}

func ParseGtid(rawGtid string) (*Gtid, error) {
	if rawGtid == "" {
		return nil, fmt.Errorf("empty GTID string")
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
