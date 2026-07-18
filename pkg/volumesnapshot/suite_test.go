package volumesnapshot

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVolumesnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volumesnapshot Suite")
}
