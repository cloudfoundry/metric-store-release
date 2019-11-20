package metricscanner_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMetricscanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metricscanner Suite")
}
