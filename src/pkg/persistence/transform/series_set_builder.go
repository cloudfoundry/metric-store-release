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
	data   map[uint64]seriesData
	count  int
	fields []string
}

func NewSeriesBuilder(fields []string) *SeriesSetBuilder {
	return &SeriesSetBuilder{
		data: make(map[uint64]seriesData),
	}
}

func (b *SeriesSetBuilder) AddSeriesPoints(points []*query.FloatPoint) {
	if len(points) == 0 {
		return
	}

	influxPoint := points[0]
	labels := LabelsFromInfluxPoint(influxPoint, b.fields)
	seriesID := labels.Hash()

	samples := make([]seriesSample, len(points))
	for i, point := range points {
		samples[i] = seriesSample{
			TimeInMilliseconds: NanosecondsToMilliseconds(point.Time),
			Value:              point.Value,
		}
	}

	sd, ok := b.data[seriesID]
	if ok {
		sd.samples = append(sd.samples, samples...)
		b.data[seriesID] = sd
	} else {
		b.data[seriesID] = seriesData{
			labels:  labels,
			samples: samples,
		}
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
