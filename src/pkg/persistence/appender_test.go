package persistence_test

import (
	"math"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/persistence" // TEMP
	"github.com/prometheus/prometheus/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type appenderTestContext struct {
	appender   storage.Appender
	adapter    *testing.SpyAdapter
	spyMetrics *testing.SpyMetricRegistrar
}

var _ = Describe("Appender", func() {
	var setup = func() appenderTestContext {
		spyAdapter := testing.NewSpyAdapter()
		spyMetrics := testing.NewSpyMetricRegistrar()

		appender := NewAppender(
			spyAdapter,
			spyMetrics,
		)

		return appenderTestContext{
			appender:   appender,
			adapter:    spyAdapter,
			spyMetrics: spyMetrics,
		}
	}

	Describe("Add()", func() {
		Context("when the metric has a null value", func() {
			It("fetches the point and its metadata", func() {
				tc := setup()

				var NegInf = math.Float64frombits(0x7FF0000000000000)

				_, err := tc.appender.Add(nil, 1, NegInf)
				Expect(err).To(MatchError("NaN float cannot be added"))
				tc.appender.Commit()

				Expect(len(tc.adapter.CommittedPoints)).To(Equal(0))
			})
		})
	})

	Describe("instrumentation", func() {
		It("updates ingress point total metric", func() {
			tc := setup()

			_, err := tc.appender.Add(nil, 1, 1.0)
			Expect(err).ToNot(HaveOccurred())
			tc.appender.Commit()

			Eventually(
				tc.spyMetrics.Fetch(metrics.MetricStoreWrittenPointsTotal),
			).Should(BeEquivalentTo(1))
		})

		It("updates the duration metrics in Commit", func() {
			tc := setup()

			_, err := tc.appender.Add(nil, 1, 1.0)
			Expect(err).ToNot(HaveOccurred())
			tc.appender.Commit()

			Eventually(
				tc.spyMetrics.FetchHistogram(metrics.MetricStoreWriteDurationSeconds),
			).Should(HaveLen(1))
		})
	})
})
