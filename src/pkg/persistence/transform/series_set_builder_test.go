package transform_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	"github.com/influxdata/influxdb/query"
	"github.com/prometheus/prometheus/pkg/labels"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SeriesSetBuilder", func() {
	Describe("creating a SeriesSet from Influx Points", func() {
		It("adds influx point(s) to a single series", func() {
			builder := transform.NewSeriesBuilder()
			builder.AddSeriesPoints(
				labels.FromMap(map[string]string{
					"__name__":  "metric_name",
					"source_id": "source_id",
				}),
				[]*query.FloatPoint{
					{
						Name:  "metric_name",
						Time:  10000000,
						Value: 99.0,
					},
					{
						Name:  "metric_name",
						Time:  15000000,
						Value: 99.0,
					},
					{
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

		It("adds influx point(s) concurrency safe", func() {
			builder := transform.NewSeriesBuilder()

			blahBlah := func(builder *transform.SeriesSetBuilder, source_id string) {
				builder.AddSeriesPoints(
					labels.Labels{},
					[]*query.FloatPoint{
						{
							Name:  "metric_name",
							Time:  10000000,
							Value: 99.0,
							Tags: query.NewTags(map[string]string{
								"__name__":  "metric_name",
								"source_id": source_id,
							}),
						},
					},
				)
			}

			go blahBlah(builder, "foo")
			go blahBlah(builder, "bar")

			Eventually(builder.Len).Should(Equal(2))
		})

		It("re-orders slices of influx point(s) that were added out of order", func() {
			builder := transform.NewSeriesBuilder()
			builder.AddSeriesPoints(
				labels.FromMap(map[string]string{
					"__name__":  "metric_name",
					"source_id": "source_id",
				}),
				[]*query.FloatPoint{
					{
						Name:  "metric_name",
						Time:  20000000,
						Value: 99.0,
						Tags: query.NewTags(map[string]string{
							"__name__":  "metric_name",
							"source_id": "source_id",
						}),
					},
				},
			)
			builder.AddSeriesPoints(
				labels.FromMap(map[string]string{
					"__name__":  "metric_name",
					"source_id": "source_id",
				}),
				[]*query.FloatPoint{
					{
						Name:  "metric_name",
						Time:  30000000,
						Value: 99.0,
						Tags: query.NewTags(map[string]string{
							"__name__":  "metric_name",
							"source_id": "source_id",
						}),
					},
				},
			)
			builder.AddSeriesPoints(
				labels.FromMap(map[string]string{
					"__name__":  "metric_name",
					"source_id": "source_id",
				}),
				[]*query.FloatPoint{
					{
						Name:  "metric_name",
						Time:  10000000,
						Value: 99.0,
						Tags: query.NewTags(map[string]string{
							"__name__":  "metric_name",
							"source_id": "source_id",
						}),
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
						{Time: 20, Value: 99.0},
						{Time: 30, Value: 99.0},
					},
				},
			))
		})
	})
})
