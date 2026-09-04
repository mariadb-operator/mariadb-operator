package binlog

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBinlog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Binlog Suite")
}
