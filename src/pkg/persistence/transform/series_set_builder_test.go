package transform_test

import (
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence/transform"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"github.com/influxdata/influxdb/query"

	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SeriesSetBuilder", func() {
	Describe("creating a SeriesSet from Influx Points", func() {
		It("adds influx point(s) to a single series", func() {
			builder := transform.NewSeriesBuilder()
			builder.AddInfluxPoint(
				&query.FloatPoint{
					Name:  "metric_name",
					Time:  10000000,
					Value: 99.0,
				},
				[]string{},
			)
			Expect(builder.Len()).To(Equal(1))

			seriesSet := builder.SeriesSet()
			series := testing.ExplodeSeriesSet(seriesSet)

			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "metric_name"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
					},
				},
			))
		})

		It("adds influx points to multiple series", func() {
			builder := transform.NewSeriesBuilder()
			builder.AddInfluxPoint(
				&query.FloatPoint{
					Name:  "metric_name",
					Time:  10000000,
					Value: 99.0,
				},
				[]string{},
			)
			Expect(builder.Len()).To(Equal(1))

			builder.AddInfluxPoint(
				&query.FloatPoint{
					Name:  "other_metric_name",
					Time:  10000000,
					Value: 99.0,
				},
				[]string{},
			)
			Expect(builder.Len()).To(Equal(2))

			seriesSet := builder.SeriesSet()
			series := testing.ExplodeSeriesSet(seriesSet)

			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "metric_name"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
					},
				},
				testing.Series{
					Labels: map[string]string{"__name__": "other_metric_name"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
					},
				},
			))
		})
	})

	Describe("creating a SeriesSet from PromQL Samples", func() {
		It("add a PromQL Sample to a single series", func() {
			builder := transform.NewSeriesBuilder()
			builder.AddPromQLSample(
				&rpc.PromQL_Sample{
					Metric: map[string]string{"__name__": "metric_name", "foo": "bar"},
					Point: &rpc.PromQL_Point{
						Time:  10,
						Value: 99.0,
					},
				},
			)
			Expect(builder.Len()).To(Equal(1))

			seriesSet := builder.SeriesSet()
			series := testing.ExplodeSeriesSet(seriesSet)

			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "metric_name", "foo": "bar"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
					},
				},
			))
		})

		It("adds PromQL Samples to multiples series", func() {
			builder := transform.NewSeriesBuilder()

			builder.AddPromQLSample(
				&rpc.PromQL_Sample{
					Metric: map[string]string{"__name__": "metric_name"},
					Point: &rpc.PromQL_Point{
						Time:  10,
						Value: 99.0,
					},
				},
			)
			Expect(builder.Len()).To(Equal(1))

			builder.AddPromQLSample(
				&rpc.PromQL_Sample{
					Metric: map[string]string{"__name__": "other_metric_name"},
					Point: &rpc.PromQL_Point{
						Time:  10,
						Value: 99.0,
					},
				},
			)
			Expect(builder.Len()).To(Equal(2))

			seriesSet := builder.SeriesSet()
			series := testing.ExplodeSeriesSet(seriesSet)

			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "metric_name"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
					},
				},
				testing.Series{
					Labels: map[string]string{"__name__": "other_metric_name"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
					},
				},
			))
		})
	})

	Describe("creating a SeriesSet from PromQL Series", func() {
		It("add a PromQL Series to a single series", func() {
			builder := transform.NewSeriesBuilder()
			builder.AddPromQLSeries(
				&rpc.PromQL_Series{
					Metric: map[string]string{"__name__": "metric_name", "foo": "bar"},
					Points: []*rpc.PromQL_Point{
						{
							Time:  10,
							Value: 99.0,
						},
						{
							Time:  11,
							Value: 99.0,
						},
					},
				},
			)
			Expect(builder.Len()).To(Equal(2))

			seriesSet := builder.SeriesSet()
			series := testing.ExplodeSeriesSet(seriesSet)

			Expect(series).To(ConsistOf(
				testing.Series{
					Labels: map[string]string{"__name__": "metric_name", "foo": "bar"},
					Points: []testing.Point{
						{Time: 10, Value: 99.0},
						{Time: 11, Value: 99.0},
					},
				},
			))
		})
	})
})
