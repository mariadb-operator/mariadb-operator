package binlog

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/utils/ptr"
)

func BinlogNum(filename string) (*int, error) {
	p := strings.LastIndexAny(filename, ".")
	if p < 0 {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	num, err := strconv.Atoi(filename[p+1:])
	if err != nil {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	return &num, nil
}

func BinlogPrefix(filename string) (*string, error) {
	p := strings.LastIndexAny(filename, ".")
	if p < 0 {
		return nil, fmt.Errorf("unexpected binlog name: %v", filename)
	}
	return ptr.To(filename[:p]), nil
}
