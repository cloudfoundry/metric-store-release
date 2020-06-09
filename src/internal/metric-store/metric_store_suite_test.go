package metric_store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetricStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MetricStore Suite")
}
