package embed

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEmbed(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Embed Suite")
}
