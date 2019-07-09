package persistence_test

import (
	"math"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/persistence" // TEMP
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	"github.com/prometheus/prometheus/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type appenderTestContext struct {
	appender storage.Appender
	adapter  *testing.SpyAdapter
}

var _ = Describe("Appender", func() {
	var setup = func() appenderTestContext {
		spyAdapter := testing.NewSpyAdapter()

		appender := NewAppender(
			spyAdapter,
		)

		return appenderTestContext{
			appender: appender,
			adapter:  spyAdapter,
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
})
