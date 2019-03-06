package metricstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetricStoreAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MetricStore Acceptance Suite")
}
