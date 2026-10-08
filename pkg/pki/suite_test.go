package pki

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPKI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PKI Suite")
}
