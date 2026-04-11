package environment

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEnvironment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Environment Suite")
}
