package config

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGaleraConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Galera Config Suite")
}
