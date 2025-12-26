package binlog

import (
	"fmt"
	"strconv"
	"strings"

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

func (b *BinlogNum) GreaterThan(other *BinlogNum) bool {
	return b.num > other.num
}

func ParseBinlogPrefix(filename string) (*string, error) {
	p := strings.LastIndexAny(filename, ".")
	if p < 0 {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	return ptr.To(filename[:p]), nil
}
