package transform

import (
	"github.com/influxdata/influxdb/query"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type seriesData struct {
	labels  labels.Labels
	samples []seriesSample
}

type SeriesSetBuilder struct {
	data  map[uint64]seriesData
	count int
}

func NewSeriesBuilder() *SeriesSetBuilder {
	return &SeriesSetBuilder{
		data: make(map[uint64]seriesData),
	}
}

func (b *SeriesSetBuilder) AddPointsForSeries(labels labels.Labels, points []*query.FloatPoint) {
	seriesID := labels.Hash()

	samples := make([]seriesSample, len(points))
	for i, point := range points {
		samples[i] = seriesSample{
			TimeInMilliseconds: NanosecondsToMilliseconds(point.Time),
			Value:              point.Value,
		}
	}

	b.data[seriesID] = seriesData{
		labels:  labels,
		samples: samples,
	}

	b.count += len(points)
}

func (builder *SeriesSetBuilder) Len() int {
	return builder.count
}

func (b *SeriesSetBuilder) SeriesSet() storage.SeriesSet {
	set := &concreteSeriesSet{
		series: []storage.Series{},
	}

	for _, data := range b.data {
		set.series = append(set.series, &concreteSeries{
			labels:  data.labels,
			samples: data.samples,
		})
	}

	return set
}
