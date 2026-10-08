package config

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMaxScaleConfigSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MaxScale Config Suite")
}
