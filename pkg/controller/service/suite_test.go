package service

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServiceController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Controller Suite")
}
