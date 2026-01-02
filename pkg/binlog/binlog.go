package binlog

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"k8s.io/utils/ptr"
)

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
		// See:
		// https://github.com/go-mysql-org/go-mysql/blob/a07c974ef5a34a8d0d7dfb543652c4ba2dec90cf/replication/parser.go#L149
		// https://github.com/wal-g/wal-g/blob/c98a8ea2d4afcb639e112164b7ce30316c4fbdb0/internal/databases/mysql/mysql_binlog.go#L76
		if err := gtidEvent.Decode(rawGtidEvent[replication.EventHeaderSize:]); err != nil {
			fmt.Printf("error decoding GTID event: %v", err)
		} else {
			// See:
			// https://github.com/go-mysql-org/go-mysql/blob/a07c974ef5a34a8d0d7dfb543652c4ba2dec90cf/replication/parser.go#L315
			// https://github.com/go-mysql-org/go-mysql/blob/a07c974ef5a34a8d0d7dfb543652c4ba2dec90cf/replication/event.go#L876
			gtidEvent.GTID.ServerID = meta.ServerId
			meta.Gtid = ptr.To(gtidEvent.GTID.String())
		}
	}

	return &meta, nil
}
