package transform_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/influxdata/influxdb/query"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SeriesSetBuilder", func() {
	Describe("creating a SeriesSet from Influx Points", func() {
		It("adds influx point(s) to a single series", func() {
			builder := transform.NewSeriesBuilder([]string{})
			builder.AddSeriesPoints(
				[]*query.FloatPoint{
					&query.FloatPoint{
						Name:  "metric_name",
						Time:  10000000,
						Value: 99.0,
						Tags: query.NewTags(map[string]string{
							"__name__":  "metric_name",
							"source_id": "source_id",
						}),
					},
					&query.FloatPoint{
						Name:  "metric_name",
						Time:  15000000,
						Value: 99.0,
					},
					&query.FloatPoint{
						Name:  "metric_name",
						Time:  20000000,
						Value: 99.0,
					},
				},
			)
			Expect(builder.Len()).To(Equal(3))

			seriesSet := builder.SeriesSet()
			series := testing.ExplodeSeriesSet(seriesSet)

			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{
						"__name__":  "metric_name",
						"source_id": "source_id",
					},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
						{Time: 15, Value: 99.0},
						{Time: 20, Value: 99.0},
					},
				},
			))
		})
	})
})
