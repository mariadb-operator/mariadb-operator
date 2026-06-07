package galera

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGaleraController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Galera Controller Suite")
}
