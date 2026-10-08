package certificate

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCertificate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Certificate Suite")
}
