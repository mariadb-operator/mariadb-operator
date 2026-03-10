package binlog

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"k8s.io/utils/ptr"
)

func ParseBinlogPrefix(filename string) (*string, error) {
	p := strings.LastIndexAny(filename, ".")
	if p < 0 {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	return ptr.To(filename[:p]), nil
}

type BinlogMetadata struct {
	Timestamp uint32
	LogPos    uint32
	ServerId  uint32
	Gtid      *string
}

func GetBinlogMetadata(filename string, logger logr.Logger) (*BinlogMetadata, error) {
	parser := replication.NewBinlogParser()
	parser.SetFlavor(mysql.MariaDBFlavor)
	parser.SetVerifyChecksum(false)
	parser.SetRawMode(true)

	meta := BinlogMetadata{}
	var rawGtidEvent []byte

	err := parser.ParseFile(filename, 0, func(e *replication.BinlogEvent) error {
		if e.Header.EventType == replication.MARIADB_GTID_EVENT {
			rawGtidEvent = e.RawData
		}
		meta.Timestamp = e.Header.Timestamp
		meta.LogPos = e.Header.LogPos
		meta.ServerId = e.Header.ServerID
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error getting binlog metadata: %v", err)
	}

	if rawGtidEvent != nil {
		gtidEvent := &replication.MariadbGTIDEvent{}
		// TODO: verify this decoding, currently not parsing domain and server id:  lastArchivedGtid: 0-0-262796
		if err := gtidEvent.Decode(rawGtidEvent[replication.EventHeaderSize:]); err != nil {
			logger.Error(err, "error decoding GTID event")
		} else {
			meta.Gtid = ptr.To(gtidEvent.GTID.String())
		}
	}

	return &meta, nil
}
