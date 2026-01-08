package binlog

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/datastructures"
	"k8s.io/utils/ptr"
)

// TODO: add API version
type BinlogIndex struct {
	// Binlogs indexed by server ID
	Binlogs map[string][]BinlogMetadata `yaml:"binlogs"`
}

func (b *BinlogIndex) Exists(serverId uint32, binlog string) bool {
	binlogs, ok := b.Binlogs[serverKey(serverId)]
	if !ok {
		return false
	}
	return datastructures.Any(binlogs, func(meta BinlogMetadata) bool {
		return meta.BinlogFilename == binlog
	})
}

func (b *BinlogIndex) Add(serverId uint32, meta BinlogMetadata) {
	if b.Binlogs == nil {
		b.Binlogs = make(map[string][]BinlogMetadata)
	}
	b.Binlogs[serverKey(serverId)] = append(b.Binlogs[serverKey(serverId)], meta)
}

func serverKey(serverId uint32) string {
	return fmt.Sprintf("server-%d", serverId)
}

type BinlogNum struct {
	filename string
	num      int
}

func ParseBinlogNum(filename string) (*BinlogNum, error) {
	p := strings.LastIndexAny(filename, ".")
	if p < 0 {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	num, err := strconv.Atoi(filename[p+1:])
	if err != nil {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	return &BinlogNum{filename: filename, num: num}, nil
}

func (b *BinlogNum) String() string {
	return fmt.Sprintf("BinlogNum{filename: %s, num: %d}", b.filename, b.num)
}

func (b *BinlogNum) LessThan(other *BinlogNum) bool {
	return b.num < other.num
}

func (b *BinlogNum) Equal(other *BinlogNum) bool {
	return b.num == other.num
}

// TODO: replace timestamps with metav1.Time
type BinlogMetadata struct {
	ServerId       uint32   `yaml:"serverId"`
	ServerVersion  string   `yaml:"serverVersion"`
	BinlogVersion  uint16   `yaml:"binlogVersion"`
	BinlogFilename string   `yaml:"binlogFilename"`
	LogPos         uint32   `yaml:"logPos"`
	FirstTimestamp uint32   `yaml:"firstTimestamp"`
	LastTimestamp  uint32   `yaml:"lastTimestamp"`
	PreviousGtids  []string `yaml:"previousGtids,omitempty"`
	FirstGtid      *string  `yaml:"firstGtid,omitempty"`
	LastGtid       *string  `yaml:"lastGtid,omitempty"`
	RotateEvent    bool     `yaml:"rotateEvent"`
	StopEvent      bool     `yaml:"stopEvent"`
}

func GetBinlogMetadata(binlogPath string, logger logr.Logger) (*BinlogMetadata, error) {
	parser := replication.NewBinlogParser()
	parser.SetFlavor(mysql.MariaDBFlavor)
	parser.SetVerifyChecksum(false)
	parser.SetRawMode(true)

	meta := BinlogMetadata{
		BinlogFilename: filepath.Base(binlogPath),
	}
	var (
		rawFormatDescriptionEvent []byte
		rawGtidListEvent          []byte
		firstRawGtidEvent         []byte
		lastRawGtidEvent          []byte
	)

	if err := parser.ParseFile(binlogPath, 0, func(e *replication.BinlogEvent) error {
		meta.ServerId = e.Header.ServerID
		meta.LogPos = e.Header.LogPos

		// See: https://mariadb.com/docs/server/reference/clientserver-protocol/replication-protocol
		switch e.Header.EventType {
		case replication.FORMAT_DESCRIPTION_EVENT:
			rawFormatDescriptionEvent = e.RawData
		case replication.MARIADB_GTID_LIST_EVENT:
			rawGtidListEvent = e.RawData
		case replication.MARIADB_GTID_EVENT:
			if firstRawGtidEvent == nil {
				firstRawGtidEvent = e.RawData
			}
			lastRawGtidEvent = e.RawData
		case replication.ROTATE_EVENT:
			meta.RotateEvent = true
		case replication.STOP_EVENT:
			meta.StopEvent = true
		}
		if meta.FirstTimestamp == 0 {
			meta.FirstTimestamp = e.Header.Timestamp
		}
		meta.LastTimestamp = e.Header.Timestamp

		return nil
	}); err != nil {
		return nil, fmt.Errorf("error getting binlog metadata: %v", err)
	}

	if rawFormatDescriptionEvent != nil {
		formatDescription := &replication.FormatDescriptionEvent{}
		if err := formatDescription.Decode(rawFormatDescriptionEvent[replication.EventHeaderSize:]); err != nil {
			return nil, fmt.Errorf("error decoding format description event: %v", err)
		}
		meta.ServerVersion = formatDescription.ServerVersion
		meta.BinlogVersion = formatDescription.Version
	}

	if rawGtidListEvent != nil {
		listEvent := &replication.MariadbGTIDListEvent{}
		if err := listEvent.Decode(rawGtidListEvent[replication.EventHeaderSize:]); err != nil {
			return nil, fmt.Errorf("error decoding GTID list event: %v", err)
		}
		prevGtids := make([]string, len(listEvent.GTIDs))
		for i, gtid := range listEvent.GTIDs {
			prevGtids[i] = gtid.String()
		}
		meta.PreviousGtids = prevGtids
	}

	if firstRawGtidEvent != nil {
		firstGtid, err := decodeGTIDEvent(firstRawGtidEvent, meta.ServerId)
		if err != nil {
			return nil, fmt.Errorf("error decoding first GTID event: %v", err)
		}
		meta.FirstGtid = ptr.To(firstGtid.GTID.String())
	}
	if lastRawGtidEvent != nil {
		lastGtid, err := decodeGTIDEvent(lastRawGtidEvent, meta.ServerId)
		if err != nil {
			return nil, fmt.Errorf("error decoding last GTID event: %v", err)
		}
		meta.LastGtid = ptr.To(lastGtid.GTID.String())
	}

	return &meta, nil
}

func decodeGTIDEvent(rawEvent []byte, serverId uint32) (*replication.MariadbGTIDEvent, error) {
	gtidEvent := &replication.MariadbGTIDEvent{}
	// See:
	// https://github.com/go-mysql-org/go-mysql/blob/a07c974ef5a34a8d0d7dfb543652c4ba2dec90cf/replication/parser.go#L149
	// https://github.com/wal-g/wal-g/blob/c98a8ea2d4afcb639e112164b7ce30316c4fbdb0/internal/databases/mysql/mysql_binlog.go#L76
	if err := gtidEvent.Decode(rawEvent[replication.EventHeaderSize:]); err != nil {
		return nil, err
	}
	// See:
	// https://github.com/go-mysql-org/go-mysql/blob/a07c974ef5a34a8d0d7dfb543652c4ba2dec90cf/replication/parser.go#L315
	// https://github.com/go-mysql-org/go-mysql/blob/a07c974ef5a34a8d0d7dfb543652c4ba2dec90cf/replication/event.go#L876
	gtidEvent.GTID.ServerID = serverId
	return gtidEvent, nil
}
