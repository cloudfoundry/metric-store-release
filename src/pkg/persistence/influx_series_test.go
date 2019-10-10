package persistence_test

import (
	"io/ioutil"

	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	"github.com/cloudfoundry/metric-store-release/src/pkg/rpc"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	"github.com/influxdata/influxdb/tsdb"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type influxSeriesTestContext struct {
	tsStore *tsdb.Store
	adapter *InfluxAdapter
}

var _ = Describe("Influx Series", func() {
	var setup = func() *influxSeriesTestContext {
		storagePath, err := ioutil.TempDir("", "metric-store")
		if err != nil {
			panic(err)
		}

		metrics := testing.NewSpyMetricRegistrar()

		tsStore, err := OpenTsStore(storagePath)
		adapter := NewInfluxAdapter(tsStore, metrics, logger.NewTestLogger())

		return &influxSeriesTestContext{
			tsStore: tsStore,
			adapter: adapter,
		}
	}

	Describe("GetSeriesSet()", func() {
		It("returns a SeriesSet containing an entry for each matching unique label combination", func() {
			tc := setup()
			tc.storePointWithLabels(10, "cpu", 1, map[string]string{"same-label": "same-value", "different-label": "value-one"})
			tc.storePointWithLabels(20, "cpu", 2, map[string]string{"same-label": "same-value", "different-label": "value-two"})
			tc.storePointWithLabels(25, "cpu", 3, map[string]string{"same-label": "same-value", "different-label": "value-one"})

			seriesSet, err := tc.adapter.GetSeriesSet([]uint64{0}, "cpu", nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(seriesSet).To(ConsistOf(
				ConsistOf(
					labels.Label{Name: "same-label", Value: "same-value"},
					labels.Label{Name: "different-label", Value: "value-one"},
				),
				ConsistOf(
					labels.Label{Name: "same-label", Value: "same-value"},
					labels.Label{Name: "different-label", Value: "value-two"},
				),
			))
		})

		It("returns a SeriesSet only containing entries for the given measurementName", func() {
			tc := setup()
			tc.storePointWithLabels(10, "cpu", 1, map[string]string{"same-label": "same-value"})
			tc.storePointWithLabels(20, "memory", 2, map[string]string{"different-label": "different-value"})

			seriesSet, err := tc.adapter.GetSeriesSet([]uint64{0}, "cpu", nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(seriesSet).To(ConsistOf(
				ConsistOf(
					labels.Label{Name: "same-label", Value: "same-value"},
				),
			))
		})

		It("returns a SeriesSet only containing entries for the given measurementName and filterConditions", func() {
			tc := setup()
			tc.storePointWithLabels(10, "cpu", 1, map[string]string{"same-label": "same-value", "filter-label": "accept"})
			tc.storePointWithLabels(10, "cpu", 1, map[string]string{"same-label": "same-value", "filter-label": "reject"})
			tc.storePointWithLabels(20, "memory", 2, map[string]string{"different-label": "different-value"})

			labelFilter := influxql.BinaryExpr{
				LHS: &influxql.VarRef{Val: "filter-label"},
				RHS: &influxql.StringLiteral{Val: "accept"},
				Op:  influxql.EQ,
			}
			seriesSet, err := tc.adapter.GetSeriesSet([]uint64{0}, "cpu", &labelFilter)
			Expect(err).ToNot(HaveOccurred())

			Expect(seriesSet).To(ConsistOf(
				ConsistOf(
					labels.Label{Name: "same-label", Value: "same-value"},
					labels.Label{Name: "filter-label", Value: "accept"},
				),
			))
		})
	})
})

func (tc *influxSeriesTestContext) storePointWithLabels(timestamp int64, name string, value float64, addLabels map[string]string) {
	point := &rpc.Point{
		Name:      name,
		Timestamp: timestamp,
		Value:     value,
		Labels:    addLabels,
	}

	tc.adapter.WritePoints([]*rpc.Point{point})
}
