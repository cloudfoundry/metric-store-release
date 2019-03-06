package system_stats_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSystemStats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "System Stats Suite")
}
