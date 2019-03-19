package metrics_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/metrics"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	var (
		m *metrics.Metrics
	)

	BeforeEach(func() {
		m = metrics.New()
	})

	It("publishes the total of a counter", func() {
		c := m.NewCounter("some_counter")
		c(99)
		c(101)

		Expect(m.Registry).To(ContainCounterMetric("some_counter", 200))
	})

	It("publishes the value of a gauge", func() {
		c := m.NewGauge("some_gauge", "some_unit")
		c(99.9)
		c(101.1)

		Expect(m.Registry).To(ContainGaugeMetric("some_gauge", "some_unit", 101.1))
	})
})
