package replication

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReplicationController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Replication Controller Suite")
}
