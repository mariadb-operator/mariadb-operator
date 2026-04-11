package statefulset

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStatefulset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Statefulset Suite")
}
